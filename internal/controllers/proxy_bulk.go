package controllers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// proxy_bulk.go — 代理批量导入与批量探测。

const (
	maxProxyImportLines   = 500
	maxProxyImportErrors  = 20
	maxProxyBatchTest     = 100
	proxyBatchTestWorkers = 6
	proxyBatchTestTimeout = 12 * time.Second
)

type ProxyImportRequest struct {
	Text    string `json:"text" binding:"required"`
	Enabled *bool  `json:"enabled"`
}

type ProxyImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors"`
}

type BatchTestProxyRequest struct {
	IDs    []string `json:"ids"`
	Target string   `json:"target"`
}

type ProxyBatchTestItem struct {
	ID     string                   `json:"id"`
	Name   string                   `json:"name"`
	Result services.ProxyTestResult `json:"result"`
}

// parseProxyLine 支持 host:port、host:port:user:pass、URL 形式，
// 以及行尾 #名称 自定义名称。空行与 # 开头的注释行返回 nil,nil。
func parseProxyLine(line string) (*models.ProxyConfig, string, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, "", nil
	}

	name := ""
	if idx := strings.LastIndex(line, "#"); idx > 0 {
		name = strings.TrimSpace(line[idx+1:])
		line = strings.TrimSpace(line[:idx])
	}
	if line == "" {
		return nil, "", fmt.Errorf("缺少代理地址")
	}

	protocol := "http"
	host, username, password := "", "", ""
	port := 0

	if strings.Contains(line, "://") {
		u, err := url.Parse(line)
		if err != nil {
			return nil, "", fmt.Errorf("地址格式错误")
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "socks5", "socks5h":
			protocol = normalizeProxyProtocol(u.Scheme)
		default:
			return nil, "", fmt.Errorf("不支持的协议 %s", u.Scheme)
		}
		host = u.Hostname()
		if u.Port() == "" {
			return nil, "", fmt.Errorf("缺少端口")
		}
		port, err = strconv.Atoi(u.Port())
		if err != nil {
			return nil, "", fmt.Errorf("端口无效")
		}
		if u.User != nil {
			username = u.User.Username()
			password, _ = u.User.Password()
		}
	} else if h, portText, err := net.SplitHostPort(line); err == nil {
		host = h
		port, _ = strconv.Atoi(portText)
	} else {
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			return nil, "", fmt.Errorf("缺少端口")
		}
		host = strings.TrimSpace(parts[0])
		var err error
		port, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, "", fmt.Errorf("端口无效")
		}
		if len(parts) >= 4 {
			username = strings.TrimSpace(parts[2])
			password = strings.TrimSpace(strings.Join(parts[3:], ":"))
		}
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return nil, "", fmt.Errorf("缺少代理地址")
	}
	if port < 1 || port > 65535 {
		return nil, "", fmt.Errorf("端口需在 1-65535 之间")
	}

	return &models.ProxyConfig{
		ID:         uuid.New().String(),
		Name:       name,
		Protocol:   protocol,
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		Enabled:    true,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}, name, nil
}

func proxyDedupKey(p *models.ProxyConfig) string {
	return strings.Join([]string{
		p.Protocol,
		strings.ToLower(strings.TrimSpace(p.Host)),
		strconv.Itoa(p.Port),
		p.Username,
	}, "\x00")
}

func uniqueProxyName(base string, used map[string]struct{}) string {
	if base == "" {
		base = "导入代理"
	}
	if _, exists := used[base]; !exists {
		used[base] = struct{}{}
		return base
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if _, exists := used[name]; !exists {
			used[name] = struct{}{}
			return name
		}
	}
}

// ImportProxies 批量导入代理。重复代理（协议+地址+端口+用户名相同）自动跳过，
// 名称冲突自动追加序号，单行失败不影响其余行。
func (pc *ProxyController) ImportProxies(c *gin.Context) {
	var req ProxyImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	lines := strings.Split(strings.ReplaceAll(req.Text, "\r\n", "\n"), "\n")
	if len(lines) > maxProxyImportLines {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, fmt.Sprintf("单次最多导入 %d 行", maxProxyImportLines)))
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	var existing []models.ProxyConfig
	if err := database.GetDB().Find(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "查询已有代理失败"))
		return
	}
	dedup := make(map[string]struct{}, len(existing))
	usedNames := make(map[string]struct{}, len(existing)+len(lines))
	for i := range existing {
		dedup[proxyDedupKey(&existing[i])] = struct{}{}
		usedNames[existing[i].Name] = struct{}{}
	}

	result := ProxyImportResult{Errors: []string{}}
	for index, line := range lines {
		proxy, name, err := parseProxyLine(line)
		if err != nil {
			result.Failed++
			if len(result.Errors) < maxProxyImportErrors {
				result.Errors = append(result.Errors, fmt.Sprintf("第 %d 行：%s", index+1, err.Error()))
			}
			continue
		}
		if proxy == nil {
			continue
		}

		key := proxyDedupKey(proxy)
		if _, ok := dedup[key]; ok {
			result.Skipped++
			continue
		}

		proxy.Name = uniqueProxyName(name, usedNames)
		proxy.Enabled = enabled
		if err := database.GetDB().Create(proxy).Error; err != nil {
			result.Failed++
			if len(result.Errors) < maxProxyImportErrors {
				result.Errors = append(result.Errors, fmt.Sprintf("第 %d 行：保存失败", index+1))
			}
			continue
		}
		dedup[key] = struct{}{}
		result.Imported++
	}

	c.JSON(http.StatusOK, models.SuccessResponse(result, "导入完成"))
}

// BatchTestProxy 批量探测代理；不传 ids 时探测全部（最多 100 个）。
func (pc *ProxyController) BatchTestProxy(c *gin.Context) {
	var req BatchTestProxyRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
			return
		}
	}

	query := database.GetDB().Order("create_time DESC").Limit(maxProxyBatchTest)
	if len(req.IDs) > 0 {
		query = query.Where("id IN ?", req.IDs)
	}
	var proxies []models.ProxyConfig
	if err := query.Find(&proxies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "查询代理失败"))
		return
	}
	if len(proxies) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "没有可探测的代理"))
		return
	}

	results := make([]ProxyBatchTestItem, len(proxies))
	sem := make(chan struct{}, proxyBatchTestWorkers)
	var wg sync.WaitGroup
	for i := range proxies {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(c.Request.Context(), proxyBatchTestTimeout)
			defer cancel()
			results[i] = ProxyBatchTestItem{
				ID:     proxies[i].ID,
				Name:   proxies[i].Name,
				Result: pc.ociService.ProxyService().Test(ctx, &proxies[i], req.Target),
			}
		}(i)
	}
	wg.Wait()

	c.JSON(http.StatusOK, models.SuccessResponse(results, "探测完成"))
}
