package services

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
)

func TestClientCacheTracksCredentialChanges(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("keys", 0700); err != nil {
		t.Fatal(err)
	}
	// Configuration providers read the key here; RSA parsing happens only when
	// creating an SDK client. No OCI request is made by this test.
	if err := os.WriteFile("keys/old.pem", []byte("old test key"), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewOCIService(nil)
	user := models.OciUser{ID: "config", OciRegion: "region", OciKeyPath: "old.pem"}
	oldClients, err := s.getCachedClients(&user)
	if err != nil {
		t.Fatal(err)
	}
	s.InvalidateClientCache(user.ID)
	// An old request may refill its cache entry after invalidation.
	if _, err := s.getCachedClients(&user); err != nil {
		t.Fatal(err)
	}
	updated := user
	updated.OciKeyPath = "new.pem"
	if _, err := s.getCachedClients(&updated); err == nil {
		t.Fatal("updated credentials reused the old key instead of reading the new file")
	}
	if err := os.WriteFile("keys/new.pem", []byte("new test key"), 0600); err != nil {
		t.Fatal(err)
	}
	newClients, err := s.getCachedClients(&updated)
	if err != nil || newClients == oldClients {
		t.Fatalf("credential revision did not create a new provider: %v", err)
	}
	again, err := s.getCachedClients(&updated)
	if err != nil || again != newClients {
		t.Fatalf("unchanged credentials did not reuse their provider: %v", err)
	}
	s.InvalidateClientCache(user.ID)
	entries := 0
	s.clientCache.Range(func(_, _ any) bool { entries++; return true })
	if entries != 0 {
		t.Fatalf("invalidation left %d credential revisions", entries)
	}
}

func TestClientCacheInjectsProxyBoundToConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("keys", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("keys/proxy.pem", []byte("proxy test key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(filepath.Join(t.TempDir(), "cache.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	proxy := models.ProxyConfig{ID: "proxy-1", Name: "p", Protocol: "http", Host: "127.0.0.1", Port: 3128, Enabled: true}
	if err := db.Create(&proxy).Error; err != nil {
		t.Fatal(err)
	}

	s := NewOCIService(nil)
	user := models.OciUser{ID: "config", OciRegion: "region", OciKeyPath: "proxy.pem", ProxyID: "proxy-1"}
	cc, err := s.getCachedClients(&user)
	if err != nil {
		t.Fatal(err)
	}
	if cc.httpClient == nil {
		t.Fatal("expected bound proxy to be injected into cached clients")
	}
	transport, ok := cc.httpClient.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		t.Fatal("expected proxy transport on injected HTTP client")
	}

	// 代理变更后应清除绑定该代理的配置缓存，下次请求重建客户端。
	bound, err := s.getCachedClients(&user)
	if err != nil || bound != cc {
		t.Fatalf("expected cached clients for unchanged proxy: %v", err)
	}
	s.InvalidateClientsByProxy("proxy-1")
	rebuilt, err := s.getCachedClients(&user)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt == cc {
		t.Fatal("expected proxy invalidation to rebuild cached clients")
	}

	// 禁用代理后回退直连，不再注入自定义 HTTP 客户端。
	if err := db.Model(&models.ProxyConfig{}).Where("id = ?", "proxy-1").Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	s.InvalidateClientsByProxy("proxy-1")
	direct, err := s.getCachedClients(&user)
	if err != nil {
		t.Fatal(err)
	}
	if direct.httpClient != nil {
		t.Fatal("expected disabled proxy to fall back to direct connection")
	}
}
