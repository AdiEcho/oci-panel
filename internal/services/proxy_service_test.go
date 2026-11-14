package services

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/adiecho/oci-panel/internal/models"
)

func TestProxyServiceSkipsDisabledOrUnsetProxy(t *testing.T) {
	s := NewProxyService()

	if client, err := s.HTTPClient(nil); err != nil || client != nil {
		t.Fatalf("nil proxy should fall back to direct connection, got %v %v", client, err)
	}
	if client, err := s.HTTPClient(&models.ProxyConfig{ID: "p", Enabled: false}); err != nil || client != nil {
		t.Fatalf("disabled proxy should fall back to direct connection, got %v %v", client, err)
	}
}

func TestProxyServiceRejectsUnsupportedProtocol(t *testing.T) {
	s := NewProxyService()
	_, err := s.HTTPClient(&models.ProxyConfig{ID: "p", Protocol: "ftp", Host: "127.0.0.1", Port: 21, Enabled: true})
	if err == nil {
		t.Fatal("expected error for unsupported proxy protocol")
	}
}

func TestProxyServiceCachesPerProxyParameters(t *testing.T) {
	s := NewProxyService()
	proxy := &models.ProxyConfig{ID: "p", Protocol: "http", Host: "127.0.0.1", Port: 8080, Enabled: true}

	first, err := s.HTTPClient(proxy)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.HTTPClient(proxy)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("expected cached client for identical proxy parameters")
	}

	proxy.Port = 8081
	changed, err := s.HTTPClient(proxy)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("expected new client after proxy port change")
	}

	s.Invalidate("p")
	invalidated, err := s.HTTPClient(proxy)
	if err != nil {
		t.Fatal(err)
	}
	if invalidated == changed {
		t.Fatal("expected new client after invalidation")
	}

	s.Invalidate("other")
	if _, err := s.HTTPClient(proxy); err != nil {
		t.Fatal(err)
	}
}

// startFakeHTTPProxy 启动一个最小 HTTP 代理：返回固定出口 IP，便于验证请求确实经代理转发。
func startFakeHTTPProxy(t *testing.T, egress string) (string, int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host == "" {
			t.Errorf("request did not use absolute-form proxy URL: %s", r.URL.String())
		}
		_, _ = w.Write([]byte(egress))
	}))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

func TestProxyServiceRoutesRequestsThroughHTTPProxy(t *testing.T) {
	host, port := startFakeHTTPProxy(t, "203.0.113.7")
	s := NewProxyService()
	proxy := &models.ProxyConfig{ID: "p", Protocol: "http", Host: host, Port: port, Enabled: true}

	client, err := s.HTTPClient(proxy)
	if err != nil {
		t.Fatal(err)
	}
	// 目标域名不可解析也不影响：HTTP 代理接收绝对 URL 自行出网。
	response, err := client.Get("http://origin.invalid/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status via proxy: %d", response.StatusCode)
	}

	result := s.Test(context.Background(), proxy, "http://origin.invalid/ip")
	if !result.Reachable || result.EgressIP != "203.0.113.7" {
		t.Fatalf("expected reachable proxy with egress IP, got %+v", result)
	}
}
