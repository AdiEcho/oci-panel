package controllers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
)

func TestParseProxyLineFormats(t *testing.T) {
	cases := []struct {
		line     string
		wantErr  bool
		protocol string
		host     string
		port     int
		username string
		password string
		name     string
	}{
		{line: "1.2.3.4:8080", protocol: "http", host: "1.2.3.4", port: 8080},
		{line: "host.example.com:3128:user:pass:with:colons", protocol: "http", host: "host.example.com", port: 3128, username: "user", password: "pass:with:colons"},
		{line: "socks5://alice:p%40ss@proxy.example.com:1080#SG", protocol: "socks5", host: "proxy.example.com", port: 1080, username: "alice", password: "p@ss", name: "SG"},
		{line: "https://proxy.example.com:8443", protocol: "https", host: "proxy.example.com", port: 8443},
		{line: "[2001:db8::1]:8080", protocol: "http", host: "2001:db8::1", port: 8080},
		{line: "# a comment"},
		{line: "   "},
		{line: "ftp://1.2.3.4:21", wantErr: true},
		{line: "no-port", wantErr: true},
		{line: "1.2.3.4:0", wantErr: true},
	}

	for _, tc := range cases {
		proxy, name, err := parseProxyLine(tc.line)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error, got %+v", tc.line, proxy)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", tc.line, err)
		}
		if proxy == nil {
			if tc.protocol != "" {
				t.Fatalf("%q: expected parsed proxy, got comment/empty", tc.line)
			}
			continue
		}
		if proxy.Protocol != tc.protocol || proxy.Host != tc.host || proxy.Port != tc.port ||
			proxy.Username != tc.username || proxy.Password != tc.password || name != tc.name {
			t.Fatalf("%q: unexpected parse result %+v name=%q", tc.line, proxy, name)
		}
	}
}

func TestImportProxiesSkipsDuplicatesAndBadLines(t *testing.T) {
	setupProxyTestDB(t)
	db := database.GetDB()
	if err := db.Create(&models.ProxyConfig{
		ID: "existing", Name: "旧代理", Protocol: "http", Host: "1.1.1.1", Port: 8080, Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	ctrl := NewProxyController(services.NewOCIService(nil))
	r := gin.New()
	r.POST("/proxy/import", ctrl.ImportProxies)

	text := "# 注释行\n" +
		"1.1.1.1:8080\n" +
		"2.2.2.2:8080\n" +
		"socks5://alice:p%40ss@3.3.3.3:1080#SG\n" +
		"https://4.4.4.4:8443\n" +
		"badline\n" +
		"4.4.4.4:8443#SG\n"
	payload, err := json.Marshal(gin.H{"text": text})
	if err != nil {
		t.Fatal(err)
	}
	response := proxyRequest(t, r, http.MethodPost, "/proxy/import", string(payload))
	data := response["data"].(map[string]any)
	if data["imported"].(float64) != 4 || data["skipped"].(float64) != 1 || data["failed"].(float64) != 1 {
		t.Fatalf("unexpected import summary: %+v", data)
	}
	if errors, ok := data["errors"].([]any); !ok || len(errors) != 1 {
		t.Fatalf("expected one error message, got %+v", data["errors"])
	}

	var socks models.ProxyConfig
	if err := db.Where("name = ?", "SG").First(&socks).Error; err != nil {
		t.Fatal(err)
	}
	if socks.Protocol != "socks5" || socks.Username != "alice" || socks.Password != "p@ss" {
		t.Fatalf("unexpected imported socks proxy: %+v", socks)
	}

	// 同一地址不同协议不视为重复；名称冲突自动追加序号。
	var renamed models.ProxyConfig
	if err := db.Where("name = ?", "SG-2").First(&renamed).Error; err != nil {
		t.Fatal(err)
	}
	if renamed.Protocol != "http" || renamed.Port != 8443 {
		t.Fatalf("unexpected renamed proxy: %+v", renamed)
	}

	second := proxyRequest(t, r, http.MethodPost, "/proxy/import", string(payload))
	again := second["data"].(map[string]any)
	if again["imported"].(float64) != 0 || again["skipped"].(float64) != 5 || again["failed"].(float64) != 1 {
		t.Fatalf("second import should only skip duplicates: %+v", again)
	}
}

func TestBatchTestProxiesReturnsPerProxyResults(t *testing.T) {
	setupProxyTestDB(t)
	db := database.GetDB()
	for _, id := range []string{"p1", "p2"} {
		// 127.0.0.1:1 立即拒绝连接，避免测试依赖外网。
		if err := db.Create(&models.ProxyConfig{
			ID: id, Name: id, Protocol: "http", Host: "127.0.0.1", Port: 1, Enabled: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	ctrl := NewProxyController(services.NewOCIService(nil))
	r := gin.New()
	r.POST("/proxy/batchTest", ctrl.BatchTestProxy)

	response := proxyRequest(t, r, http.MethodPost, "/proxy/batchTest", `{"ids":["p1","p2"]}`)
	items, ok := response["data"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("unexpected batch test response: %+v", response)
	}
	for _, item := range items {
		result := item.(map[string]any)["result"].(map[string]any)
		if result["reachable"] != false || result["message"] == "" {
			t.Fatalf("expected unreachable proxy with message, got %+v", result)
		}
	}
}
