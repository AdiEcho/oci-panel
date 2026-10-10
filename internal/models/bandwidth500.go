package models

import "time"

// Bandwidth500Resource 表示由 500Mbps 功能创建、采用或借用的 OCI 物理资源记录
type Bandwidth500Resource struct {
	ID                    string    `gorm:"primaryKey;column:id;type:varchar(64)" json:"id"`
	ResourceType          string    `gorm:"column:resource_type;index;type:varchar(64)" json:"resourceType"` // nat_gateway, nlb, route_table
	Ocid                  string    `gorm:"column:ocid;uniqueIndex;type:varchar(255)" json:"ocid"`
	TenantID              string    `gorm:"column:tenant_id;index;type:varchar(255)" json:"tenantId"`
	Region                string    `gorm:"column:region;index;type:varchar(64)" json:"region"`
	ResourceCompartmentID string    `gorm:"column:resource_compartment_id;type:varchar(255)" json:"resourceCompartmentId"`
	VcnID                 string    `gorm:"column:vcn_id;index;type:varchar(255)" json:"vcnId"`
	PoolID                string    `gorm:"column:pool_id;index;type:varchar(255)" json:"poolId"`
	Origin                string    `gorm:"column:origin;type:varchar(32)" json:"origin"` // created, adopted, borrowed
	CanDelete             bool      `gorm:"column:can_delete" json:"canDelete"`
	State                 string    `gorm:"column:state;type:varchar(32)" json:"state"` // active, deleted
	Version               int       `gorm:"column:version" json:"version"`
	CreatedAt             time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt             time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

// Bandwidth500Binding 表示实例与 500Mbps 网络配置（NAT/NLB/路由）的绑定关系
type Bandwidth500Binding struct {
	ID               string    `gorm:"primaryKey;column:id;type:varchar(64)" json:"id"`
	InstanceID       string    `gorm:"column:instance_id;uniqueIndex;type:varchar(255)" json:"instanceId"`
	UserID           string    `gorm:"column:user_id;index;type:varchar(64)" json:"userId"`
	TenantID         string    `gorm:"column:tenant_id;index;type:varchar(255)" json:"tenantId"`
	Region           string    `gorm:"column:region;index;type:varchar(64)" json:"region"`
	VcnID            string    `gorm:"column:vcn_id;index;type:varchar(255)" json:"vcnId"`
	Mode             string    `gorm:"column:mode;type:varchar(32)" json:"mode"` // dedicated, shared-ports
	VnicID           string    `gorm:"column:vnic_id;type:varchar(255)" json:"vnicId"`
	PrivateIP        string    `gorm:"column:private_ip;type:varchar(64)" json:"privateIp"`
	NatGatewayID     string    `gorm:"column:nat_gateway_id;type:varchar(255)" json:"natGatewayId"`
	NlbID            string    `gorm:"column:nlb_id;type:varchar(255)" json:"nlbId"`
	RouteTableID     string    `gorm:"column:route_table_id;type:varchar(255)" json:"routeTableId"`
	OriginalSnapshot string    `gorm:"column:original_snapshot;type:text" json:"originalSnapshot"` // JSON: 原 VNIC 路由表 ID、原 SkipSourceDestCheck、原 NSG 等
	TargetEndpoints  string    `gorm:"column:target_endpoints;type:text" json:"targetEndpoints"`   // JSON: 端点映射列表
	Status           string    `gorm:"column:status;type:varchar(32);index" json:"status"`         // enabled, disabled, repair-required, migrating, enabling, disabling
	ObservedHealth   string    `gorm:"column:observed_health;type:varchar(32)" json:"observedHealth"`
	ObservedIP       string    `gorm:"column:observed_ip;type:varchar(64)" json:"observedIp"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

// Bandwidth500Operation 记录异步操作（开启、关闭、迁移等）与进度
type Bandwidth500Operation struct {
	ID         string    `gorm:"primaryKey;column:id;type:varchar(64)" json:"id"`
	InstanceID string    `gorm:"column:instance_id;index;type:varchar(255)" json:"instanceId"`
	Action     string    `gorm:"column:action;type:varchar(32)" json:"action"`       // enable, disable, migrate
	Status     string    `gorm:"column:status;index;type:varchar(32)" json:"status"` // queued, running, succeeded, failed, rolled-back, repair-required
	Progress   int       `gorm:"column:progress" json:"progress"`                    // 0-100
	Message    string    `gorm:"column:message;type:text" json:"message"`
	Error      string    `gorm:"column:error;type:text" json:"error"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

// Bandwidth500Endpoint 表示单个端口映射
type Bandwidth500Endpoint struct {
	ID          string `json:"id"`
	Protocol    string `json:"protocol"` // TCP, UDP
	ListenPort  int    `json:"listenPort"`
	BackendPort int    `json:"backendPort"`
}

// Bandwidth500OriginalSnapshot 原始配置快照
type Bandwidth500OriginalSnapshot struct {
	RouteTableID        *string  `json:"routeTableId"`
	SkipSourceDestCheck *bool    `json:"skipSourceDestCheck"`
	NsgIDs              []string `json:"nsgIds,omitempty"`
	HadExplicitRoute    bool     `json:"hadExplicitRoute"`
}

// Bandwidth500Summary 用于在实例列表与详情中统一呈现 500Mbps 状态及访问入口
type Bandwidth500Summary struct {
	Status              string                 `json:"status"`  // enabled, disabled, enabling, disabling, repair-required, unknown
	Enabled             *bool                  `json:"enabled"` // true | false | nil
	Mode                string                 `json:"mode"`    // dedicated, shared-ports
	Source              string                 `json:"source"`  // binding, legacy-verified, unknown
	NlbID               string                 `json:"nlbId,omitempty"`
	AccessIP            string                 `json:"accessIp,omitempty"`
	NatEgressIP         string                 `json:"natEgressIp,omitempty"`
	Endpoints           []Bandwidth500Endpoint `json:"endpoints,omitempty"`
	Health              string                 `json:"health"`       // healthy, unhealthy, unknown, degraded
	Connectivity        string                 `json:"connectivity"` // unverified, verified
	CanEnable           bool                   `json:"canEnable"`
	CanDisable          bool                   `json:"canDisable"`
	ActionBlockedReason string                 `json:"actionBlockedReason,omitempty"`
	OperationID         string                 `json:"operationId,omitempty"`
	StatusVersion       string                 `json:"statusVersion,omitempty"`
	ObservedAt          string                 `json:"observedAt,omitempty"`
	Freshness           string                 `json:"freshness"` // fresh, stale
	SharedMemberCount   int                    `json:"sharedMemberCount,omitempty"`
	CanMigrateToShared  bool                   `json:"canMigrateToShared,omitempty"`
}
