package services

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adiecho/oci-panel/internal/models"
	xproxy "golang.org/x/net/proxy"
)

// proxy_service.go — 代理传输层：把 HTTP/HTTPS/SOCKS5 代理转换成 http.Client，
// 供 OCI SDK 客户端按配置注入（SDK 的 BaseClient.HTTPClient 接受任意 http.Client）。
// 传输层按代理参数缓存，参数变化自然形成新 key，旧条目通过 Invalidate 清理。

const (
	proxyDialTimeout = 15 * time.Second
	proxyTestTimeout = 20 * time.Second
	defaultTestURL   = "https://api.ipify.org"
)

type ProxyService struct {
	mu      sync.Mutex
	clients map[string]*http.Client
}

func NewProxyService() *ProxyService {
	return &ProxyService{clients: make(map[string]*http.Client)}
}

func proxyCacheKey(p *models.ProxyConfig) string {
	return strings.Join([]string{
		p.ID,
		p.Protocol,
		strings.ToLower(strings.TrimSpace(p.Host)),
		strconv.Itoa(p.Port),
		p.Username,
		p.Password,
	}, "\x00")
}

func ProxyAddress(p *models.ProxyConfig) string {
	return net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
}

// HTTPClient 返回经由代理转发的 HTTP 客户端。
// 代理为空、未配置 ID 或已禁用时返回 (nil, nil)，调用方保持 OCI SDK 默认传输行为。
func (s *ProxyService) HTTPClient(p *models.ProxyConfig) (*http.Client, error) {
	if p == nil || p.ID == "" || !p.Enabled {
		return nil, nil
	}

	key := proxyCacheKey(p)
	s.mu.Lock()
	defer s.mu.Unlock()

	if cached, ok := s.clients[key]; ok {
		return cached, nil
	}

	transport, err := buildProxyTransport(p)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Transport: transport}
	s.clients[key] = client
	return client, nil
}

// Invalidate 清理代理传输缓存；proxyID 为空时清空全部。
func (s *ProxyService) Invalidate(proxyID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key := range s.clients {
		if proxyID == "" || strings.HasPrefix(key, proxyID+"\x00") {
			delete(s.clients, key)
		}
	}
}

func buildProxyTransport(p *models.ProxyConfig) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	addr := ProxyAddress(p)

	switch strings.ToLower(p.Protocol) {
	case "http", "https":
		u := &url.URL{Scheme: strings.ToLower(p.Protocol), Host: addr}
		if p.Username != "" || p.Password != "" {
			u.User = url.UserPassword(p.Username, p.Password)
		}
		transport.Proxy = http.ProxyURL(u)
	case "socks5":
		var auth *xproxy.Auth
		if p.Username != "" || p.Password != "" {
			auth = &xproxy.Auth{User: p.Username, Password: p.Password}
		}
		dialer, err := xproxy.SOCKS5("tcp", addr, auth, &net.Dialer{
			Timeout:   proxyDialTimeout,
			KeepAlive: 30 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create socks5 dialer: %w", err)
		}
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			type contextDialer interface {
				DialContext(ctx context.Context, network, address string) (net.Conn, error)
			}
			if cd, ok := dialer.(contextDialer); ok {
				return cd.DialContext(ctx, network, address)
			}
			return dialer.Dial(network, address)
		}
		// 自定义拨号无法复用默认 HTTP/2 链路，显式关闭以避免协商失败。
		transport.ForceAttemptHTTP2 = false
	default:
		return nil, fmt.Errorf("unsupported proxy protocol: %s", p.Protocol)
	}

	return transport, nil
}

// ProxyTestResult 代理连通性测试结果。
type ProxyTestResult struct {
	Reachable bool   `json:"reachable"`
	LatencyMs int64  `json:"latencyMs"`
	EgressIP  string `json:"egressIp"`
	Message   string `json:"message"`
}

// Test 通过代理请求测试地址（默认返回出口 IP 的纯文本接口），
// 先探测代理端口连通性，便于区分「代理不可达」与「目标站点不可达」。
func (s *ProxyService) Test(ctx context.Context, p *models.ProxyConfig, target string) ProxyTestResult {
	if p == nil {
		return ProxyTestResult{Message: "代理不存在"}
	}
	if !p.Enabled {
		return ProxyTestResult{Message: "代理已禁用"}
	}
	if strings.TrimSpace(target) == "" {
		target = defaultTestURL
	}

	dialer := &net.Dialer{Timeout: proxyDialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", ProxyAddress(p))
	if err != nil {
		return ProxyTestResult{Message: fmt.Sprintf("代理端口不可达：%v", err)}
	}
	_ = conn.Close()

	client, err := s.HTTPClient(p)
	if err != nil {
		return ProxyTestResult{Message: fmt.Sprintf("代理初始化失败：%v", err)}
	}

	reqCtx, cancel := context.WithTimeout(ctx, proxyTestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return ProxyTestResult{Message: fmt.Sprintf("测试地址无效：%v", err)}
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return ProxyTestResult{LatencyMs: latency, Message: fmt.Sprintf("代理请求失败：%v", err)}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return ProxyTestResult{LatencyMs: latency, Message: fmt.Sprintf("代理返回 HTTP %d", resp.StatusCode)}
	}

	return ProxyTestResult{
		Reachable: true,
		LatencyMs: latency,
		EgressIP:  strings.TrimSpace(string(body)),
		Message:   "连接成功",
	}
}
