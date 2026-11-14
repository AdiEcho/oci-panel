package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
)

func setupProxyTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "proxy.db")); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.GetDB().DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
}

func proxyRequest(t *testing.T, r *gin.Engine, method, path, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid response %q: %v", recorder.Body.String(), err)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	return payload
}

func TestProxyCRUDAndBatchBind(t *testing.T) {
	setupProxyTestDB(t)
	db := database.GetDB()
	for _, id := range []string{"config-a", "config-b"} {
		if err := db.Create(&models.OciUser{ID: id, Username: id}).Error; err != nil {
			t.Fatal(err)
		}
	}

	ctrl := NewProxyController(services.NewOCIService(nil))
	r := gin.New()
	r.POST("/proxy/create", ctrl.CreateProxy)
	r.POST("/proxy/update", ctrl.UpdateProxy)
	r.POST("/proxy/page", ctrl.ProxyPage)
	r.POST("/proxy/batchBind", ctrl.BatchBindProxy)
	r.POST("/proxy/delete", ctrl.DeleteProxy)

	createBody := `{"name":"sg-http","protocol":"HTTP","host":"10.0.0.1","port":8080,"username":"u","password":"secret","remark":"test"}`
	create := proxyRequest(t, r, http.MethodPost, "/proxy/create", createBody)
	data := create["data"].(map[string]any)
	if data["protocol"] != "http" || data["hasAuth"] != true {
		t.Fatalf("unexpected created proxy: %+v", data)
	}
	if _, leaked := data["password"]; leaked {
		t.Fatal("create response must not expose proxy password")
	}
	proxyID := data["id"].(string)

	var stored models.ProxyConfig
	if err := db.Where("id = ?", proxyID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Password != "secret" {
		t.Fatalf("expected stored password, got %q", stored.Password)
	}

	// 密码留空表示保持原值，避免编辑其他字段时丢失凭据。
	updateBody := `{"id":"` + proxyID + `","name":"sg-http","protocol":"http","host":"10.0.0.2","port":8081}`
	proxyRequest(t, r, http.MethodPost, "/proxy/update", updateBody)
	if err := db.Where("id = ?", proxyID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Host != "10.0.0.2" || stored.Port != 8081 || stored.Password != "secret" {
		t.Fatalf("unexpected proxy after update: %+v", stored)
	}

	bind := proxyRequest(t, r, http.MethodPost, "/proxy/batchBind",
		`{"configIds":["config-a","config-b"],"proxyId":"`+proxyID+`"}`)
	if bind["data"].(map[string]any)["updated"].(float64) != 2 {
		t.Fatalf("expected 2 bound configs, got %+v", bind)
	}
	var bound int64
	db.Model(&models.OciUser{}).Where("proxy_id = ?", proxyID).Count(&bound)
	if bound != 2 {
		t.Fatalf("expected 2 configs bound in database, got %d", bound)
	}

	page := proxyRequest(t, r, http.MethodPost, "/proxy/page", `{"page":1,"pageSize":10}`)
	list := page["data"].(map[string]any)["list"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["bindCount"].(float64) != 2 {
		t.Fatalf("expected bind count 2 in page response, got %+v", list)
	}

	// 解绑：空代理 ID 表示改为直连。
	proxyRequest(t, r, http.MethodPost, "/proxy/batchBind", `{"configIds":["config-a"],"proxyId":""}`)
	var unbind models.OciUser
	db.Where("id = ?", "config-a").First(&unbind)
	if unbind.ProxyID != "" {
		t.Fatalf("expected config-a unbound, got %q", unbind.ProxyID)
	}

	proxyRequest(t, r, http.MethodPost, "/proxy/delete", `{"ids":["`+proxyID+`"]}`)
	if err := db.Where("id = ?", proxyID).First(&stored).Error; err == nil {
		t.Fatal("expected proxy to be deleted")
	}
	var orphan models.OciUser
	db.Where("id = ?", "config-b").First(&orphan)
	if orphan.ProxyID != "" {
		t.Fatalf("expected bound config to be unbound on delete, got %q", orphan.ProxyID)
	}
}
