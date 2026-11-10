package services

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adiecho/oci-panel/internal/database"
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

func TestReset500MbpsStatus(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_reset.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	bwService := &Bandwidth500Service{}
	user := &models.OciUser{ID: "user-1"}

	// 1. 无记录重置应直接成功（幂等）
	if err := bwService.Reset500MbpsStatus(context.Background(), user, "inst-none"); err != nil {
		t.Errorf("expected nil for non-existing binding, got %v", err)
	}

	// 2. enabling 状态不允许直接重置
	bindingEnabling := models.Bandwidth500Binding{
		ID:         "b-1",
		InstanceID: "inst-enabling",
		Status:     StatusEnabling,
	}
	database.GetDB().Create(&bindingEnabling)
	if err := bwService.Reset500MbpsStatus(context.Background(), user, "inst-enabling"); err == nil {
		t.Error("expected error resetting enabling binding, got nil")
	}

	// 3. enabled 状态不允许通过 reset 重置（需走 disable 流程）
	bindingEnabled := models.Bandwidth500Binding{
		ID:         "b-2",
		InstanceID: "inst-enabled",
		Status:     StatusEnabled,
	}
	database.GetDB().Create(&bindingEnabled)
	if err := bwService.Reset500MbpsStatus(context.Background(), user, "inst-enabled"); err == nil {
		t.Error("expected error resetting enabled binding, got nil")
	}

	// 4. repair-required 或 failed 状态允许重置清理
	bindingFailed := models.Bandwidth500Binding{
		ID:         "b-3",
		InstanceID: "inst-failed",
		Status:     StatusRepairRequired,
		LastError:  "NLB_QUOTA_EXCEEDED: 网络负载均衡器(NLB)配额超限",
	}
	database.GetDB().Create(&bindingFailed)
	if err := bwService.Reset500MbpsStatus(context.Background(), user, "inst-failed"); err != nil {
		t.Errorf("expected success resetting failed binding, got %v", err)
	}

	// 验证记录已删除
	var check models.Bandwidth500Binding
	if err := database.GetDB().Where("instance_id = ?", "inst-failed").First(&check).Error; err == nil {
		t.Error("expected binding to be deleted after reset")
	}
}

func TestNlbPoolAssignmentLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_elb_pool.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	bwService := &Bandwidth500Service{}
	user := &models.OciUser{ID: "user-1"}

	// 1. 无成员时应可分配、可删除
	if got := bwService.countEnabledMembers("nlb-1"); got != 0 {
		t.Fatalf("expected 0 members, got %d", got)
	}

	// 2. 开启成功后将预建 ELB 标记为已分配
	bwService.markPoolAssigned(user, "nlb-1", "inst-1", "vcn-1")
	var pool models.Bandwidth500Pool
	if err := database.GetDB().Where("nlb_id = ?", "nlb-1").First(&pool).Error; err != nil {
		t.Fatalf("expected pool record created, got %v", err)
	}
	if pool.Status != PoolStatusAssigned || pool.InstanceID != "inst-1" {
		t.Errorf("expected assigned to inst-1, got status=%s instance=%s", pool.Status, pool.InstanceID)
	}
	if pool.Origin != OriginAdopted {
		t.Errorf("expected adopted origin for auto-created record, got %s", pool.Origin)
	}

	// 3. enabled 绑定使成员数变为 1
	binding := models.Bandwidth500Binding{ID: "b-1", InstanceID: "inst-1", NlbID: "nlb-1", Status: StatusEnabled}
	database.GetDB().Create(&binding)
	if got := bwService.countEnabledMembers("nlb-1"); got != 1 {
		t.Fatalf("expected 1 member, got %d", got)
	}

	// 4. 仍有成员时关闭不回收
	bwService.releasePoolNlb("nlb-1")
	database.GetDB().Where("nlb_id = ?", "nlb-1").First(&pool)
	if pool.Status != PoolStatusAssigned {
		t.Errorf("expected pool stay assigned while member exists, got %s", pool.Status)
	}

	// 5. 绑定停用后回池
	binding.Status = StatusDisabled
	database.GetDB().Save(&binding)
	bwService.releasePoolNlb("nlb-1")
	database.GetDB().Where("nlb_id = ?", "nlb-1").First(&pool)
	if pool.Status != PoolStatusAvailable || pool.InstanceID != "" {
		t.Errorf("expected pool released, got status=%s instance=%s", pool.Status, pool.InstanceID)
	}

	// 6. 云端 NLB 删除后清理记录
	bwService.deletePoolRecord("nlb-1")
	if err := database.GetDB().Where("nlb_id = ?", "nlb-1").First(&pool).Error; err == nil {
		t.Error("expected pool record deleted")
	}
}

func TestWithinPoolGrace(t *testing.T) {
	// 刚创建：处于宽限期，不应被判为云端丢失
	if !withinPoolGrace(time.Now()) {
		t.Error("expected fresh pool record to be within grace")
	}
	// 超过宽限期：允许判定丢失
	if withinPoolGrace(time.Now().Add(-poolProvisionGrace - time.Minute)) {
		t.Error("expected old pool record to be out of grace")
	}
	// 零值时间：保守判定为超期
	if withinPoolGrace(time.Time{}) {
		t.Error("expected zero time to be out of grace")
	}
}
