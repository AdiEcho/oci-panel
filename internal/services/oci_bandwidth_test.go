package services

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adiecho/oci-panel/internal/models"
)

type mockServiceError struct {
	statusCode   int
	code         string
	message      string
	opcRequestID string
}

func (m mockServiceError) GetHTTPStatusCode() int  { return m.statusCode }
func (m mockServiceError) GetCode() string         { return m.code }
func (m mockServiceError) GetMessage() string      { return m.message }
func (m mockServiceError) GetOpcRequestID() string { return m.opcRequestID }
func (m mockServiceError) Error() string {
	return m.message
}

func TestClassifyOCIError_QuotaExceeded(t *testing.T) {
	err := mockServiceError{
		statusCode:   400,
		code:         "LimitExceeded",
		message:      "The following service limits were exceeded: max-nlb-flexible-count",
		opcRequestID: "req-12345",
	}

	classified := ClassifyOCIError(err)
	if classified == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := classified.Error()
	if !strings.Contains(errMsg, "NLB_QUOTA_EXCEEDED") {
		t.Errorf("expected NLB_QUOTA_EXCEEDED in error, got %s", errMsg)
	}
	if !strings.Contains(errMsg, "max-nlb-flexible-count") {
		t.Errorf("expected max-nlb-flexible-count in error, got %s", errMsg)
	}
	if !strings.Contains(errMsg, "req-12345") {
		t.Errorf("expected request id in error, got %s", errMsg)
	}
}

func TestClassifyOCIError_OtherError(t *testing.T) {
	err := mockServiceError{
		statusCode:   404,
		code:         "NotAuthorizedOrNotFound",
		message:      "Resource not found",
		opcRequestID: "req-67890",
	}

	classified := ClassifyOCIError(err)
	if classified == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := classified.Error()
	if strings.Contains(errMsg, "NLB_QUOTA_EXCEEDED") {
		t.Errorf("should not be NLB_QUOTA_EXCEEDED, got %s", errMsg)
	}
	if !strings.Contains(errMsg, "NotAuthorizedOrNotFound") {
		t.Errorf("expected original code in error, got %s", errMsg)
	}
}

func TestClassifyOCIError_NilAndNormalError(t *testing.T) {
	if ClassifyOCIError(nil) != nil {
		t.Error("expected nil for nil error")
	}

	normalErr := errors.New("database connection failed")
	if ClassifyOCIError(normalErr) != normalErr {
		t.Error("expected normal error untouched")
	}
}

func TestShortInstanceHash(t *testing.T) {
	inst1 := "ocid1.instance.oc1..inst1"
	inst2 := "ocid1.instance.oc1..inst2"

	h1 := shortInstanceHash(inst1)
	h2 := shortInstanceHash(inst2)

	if len(h1) != 8 || len(h2) != 8 {
		t.Fatalf("expected hash length 8, got %d and %d", len(h1), len(h2))
	}
	if h1 == h2 {
		t.Fatalf("different instances should have different hash, got %s and %s", h1, h2)
	}

	// 幂等性
	if shortInstanceHash(inst1) != h1 {
		t.Fatal("hash should be deterministic")
	}
}

func TestVcnLock_Mutex(t *testing.T) {
	bwService := &Bandwidth500Service{}
	lock1 := bwService.getVcnLock("tenant1", "us-ashburn-1", "vcn1")
	lock2 := bwService.getVcnLock("tenant1", "us-ashburn-1", "vcn1")
	lockDiff := bwService.getVcnLock("tenant1", "us-ashburn-1", "vcn2")

	if lock1 != lock2 {
		t.Fatal("same tenant, region, vcn should share the same lock pointer")
	}
	if lock1 == lockDiff {
		t.Fatal("different vcn should have different lock pointer")
	}

	// 验证互斥性
	acquired := false
	lock1.Lock()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		lock2.Lock()
		acquired = true
		lock2.Unlock()
	}()

	time.Sleep(50 * time.Millisecond)
	if acquired {
		t.Fatal("lock2 should have been blocked by lock1")
	}

	lock1.Unlock()
	wg.Wait()
	if !acquired {
		t.Fatal("lock2 should have acquired after lock1 released")
	}
}

func TestPortConflictDetection(t *testing.T) {
	existingMappings := []models.Bandwidth500Endpoint{
		{ID: "ssh-1", Protocol: "TCP", ListenPort: 2202, BackendPort: 22},
		{ID: "web-1", Protocol: "TCP", ListenPort: 8080, BackendPort: 80},
		{ID: "udp-1", Protocol: "UDP", ListenPort: 5353, BackendPort: 53},
	}

	// 测试同协议同监听端口冲突
	newReq1 := models.Bandwidth500Endpoint{ID: "ssh-2", Protocol: "TCP", ListenPort: 2202, BackendPort: 22}
	conflictFound := false
	for _, em := range existingMappings {
		if strings.EqualFold(em.Protocol, newReq1.Protocol) && em.ListenPort == newReq1.ListenPort {
			conflictFound = true
			break
		}
	}
	if !conflictFound {
		t.Fatal("expected conflict for same protocol and listen port")
	}

	// 测试不同协议相同监听端口允许
	newReq2 := models.Bandwidth500Endpoint{ID: "udp-2202", Protocol: "UDP", ListenPort: 2202, BackendPort: 22}
	conflictFound2 := false
	for _, em := range existingMappings {
		if strings.EqualFold(em.Protocol, newReq2.Protocol) && em.ListenPort == newReq2.ListenPort {
			conflictFound2 = true
			break
		}
	}
	if conflictFound2 {
		t.Fatal("different protocol with same port should be allowed")
	}
}
