import { get, post, type PageResult, type ValueLabel } from './http'

/** OCI 配置（账号凭据）列表项。 */
export interface ConfigItem {
  id: string
  username: string
  tenantName?: string
  tenantCreateTime?: string
  ociRegion: string
  ociUserId?: string
  ociFingerprint?: string
  ociTenantId?: string
  ociKeyPath?: string
  proxyId?: string
  proxyName?: string
  instanceCount?: number
  runningInstances?: number
}

/** 配置详情（基础信息；其余字段由后端补充，故带索引签名保持宽松）。 */
export interface ConfigDetails {
  userId: string
  username: string
  region: string
  fingerprint: string
  keyPath: string
  [key: string]: unknown
}

export interface Bandwidth500Endpoint {
  id: string
  protocol: string
  listenPort: number
  backendPort: number
}

export interface Bandwidth500Summary {
  status: string // enabled, disabled, enabling, disabling, repair-required, failed, unknown
  enabled: boolean | null
  mode?: string // dedicated, shared-ports
  source?: string // binding, legacy-verified, unknown
  nlbId?: string
  accessIp?: string
  natEgressIp?: string
  endpoints?: Bandwidth500Endpoint[]
  health?: string
  connectivity?: string
  canEnable: boolean
  canDisable: boolean
  actionBlockedReason?: string
  lastError?: string
  operationId?: string
  operationStatus?: string
  operationMessage?: string
  statusVersion?: string
  observedAt?: string
  freshness?: string
  sharedMemberCount?: number
  canMigrateToShared?: boolean
}

/** 500Mbps 所用 ELB（OCI 网络负载均衡器）池记录。 */
export interface ElbPoolItem {
  id: string
  nlbId: string
  nlbName: string
  accessIp?: string
  vcnId: string
  vcnName?: string
  subnetId?: string
  compartmentId?: string
  mode: string // shared-ports, dedicated
  origin?: string // created, adopted, borrowed
  status: string // available, assigned, missing, failed
  instanceId?: string
  instanceName?: string
  memberCount?: number
  canDelete: boolean
  canAssign: boolean
  lastError?: string
  createdAt?: string
  observedAt?: string
}

export interface Bandwidth500PlanResponse {
  planId: string
  instanceId: string
  mode: string
  currentStatus: string
  vcnId: string
  subnetId: string
  canReuseNat: boolean
  natGatewayId?: string
  canReuseNlb: boolean
  nlbId?: string
  nlbAccessIp?: string
  effectiveMappings: Bandwidth500Endpoint[]
  portConflicts?: string[]
  requiresMigration: boolean
  migrationInstanceId?: string
  existingNlbCount: number
  quotaRisk: boolean
  warnings?: string[]
  supported: boolean
  reason?: string
}

/** 实例信息（详情页实例列表项）。 */
export interface InstanceInfo {
  id: string
  displayName: string
  state: string
  shape: string
  ocpus: number
  memory: number
  bootVolumeSize?: number
  bootVolumeVpu?: number
  region: string
  publicIps?: string[]
  ipv6?: string
  imageName?: string
  bandwidth500?: Bandwidth500Summary
}

/** 引导卷信息（字段较多且前端松散使用，保持宽松）。 */
export interface VolumeInfo {
  id: string
  displayName: string
  [key: string]: unknown
}

/** VCN 信息（含子网，字段较多，保持宽松）。 */
export interface VCNInfo {
  id: string
  displayName: string
  [key: string]: unknown
}

/** 租户信息（区域/用户列表/密码策略等，字段较多，保持宽松）。 */
export interface TenantInfo {
  name?: string
  id?: string
  [key: string]: unknown
}

/** 规格按需单价（USD/小时），由前端按 CPU/内存估算。 */
export interface ShapePrice {
  currency: string
  ocpuHourly: number
  memoryGbHourly: number
  gpuHourly: number
  nvmeTbHourly: number
  billedBy: string
  source: string
  updatedAt: string
}

/** 可创建实例的规格（对齐 Oracle 控制台规格卡片）。 */
export interface ShapeInfo {
  shape: string
  architecture: string
  processorDescription: string
  ocpus: number
  memoryInGBs: number
  networkBandwidthInGbps: number
  isFlex: boolean
  minOcpus: number
  maxOcpus: number
  minMemoryInGBs: number
  maxMemoryInGBs: number
  localDisks: number
  localDiskSizeInGBs: number
  gpus: number
  gpuDescription: string
  freeTier: boolean
  freeTierMaxOcpus: number
  freeTierMaxMemoryInGBs: number
  price?: ShapePrice
}

/** 可用镜像。 */
export interface ImageInfo {
  id: string
  operatingSystem: string
  operatingSystemVersion: string
}

/** 流量查询条件。 */
export interface TrafficCondition {
  regions?: ValueLabel[]
  instances: ValueLabel[]
}

/** 流量数据（按时间点的入站/出站序列，单位 MB）。 */
export interface TrafficData {
  time: string[]
  inbound: string[]
  outbound: string[]
}

/** VCN 安全列表（入/出站规则，字段由后端定义，保持宽松）。 */
export interface SecurityListData {
  ingressRules?: unknown[]
  egressRules?: unknown[]
  [key: string]: unknown
}

/** 添加安全规则入参。 */
export interface AddSecurityRuleReq {
  configId: string
  vcnId?: string
  isIngress: boolean
  protocol: string
  source?: string
  destination?: string
  portMin?: number
  portMax?: number
  description?: string
}

/** 配置（账号）与其资源（实例/卷/VCN/租户/镜像/流量）相关接口。 */
export const ociApi = {
  userPage: (req: { page: number; pageSize: number; username?: string }) =>
    post<PageResult<ConfigItem>>('/oci/userPage', req),
  uploadKey: (formData: FormData) =>
    post<string>('/oci/uploadKey', formData, { headers: { 'Content-Type': 'multipart/form-data' } }),
  addCfg: (req: {
    username: string
    tenantName: string
    ociTenantId: string
    ociUserId: string
    ociFingerprint: string
    ociRegion: string
    ociKeyPath: string
    proxyId?: string
  }) => post('/oci/addCfg', req),
  updateCfgName: (req: { id: string; username: string; ociKeyPath?: string; proxyId?: string }) =>
    post('/oci/updateCfgName', req),
  removeCfg: (ids: string[]) => post('/oci/removeCfg', { ids }),

  details: (configId: string) => post<ConfigDetails>('/oci/details', { configId }),
  detailsInstances: (req: { configId: string; clearCache?: boolean }) =>
    post<InstanceInfo[]>('/oci/details/instances', req),
  detailsVolumes: (req: { configId: string; clearCache?: boolean }) => post<VolumeInfo[]>('/oci/details/volumes', req),
  detailsVCNs: (req: { configId: string; clearCache?: boolean }) => post<VCNInfo[]>('/oci/details/vcns', req),

  tenantInfo: (req: { configId: string; clearCache?: boolean }) => post<TenantInfo>('/oci/tenant/info', req),
  updatePwdEx: (req: { cfgId: string; passwordExpiresAfter: number }) => post('/oci/tenant/updatePwdEx', req),
  updateUserInfo: (req: { ociCfgId: string; userId: string; email: string; dbUserName: string; description: string }) =>
    post('/oci/tenant/updateUserInfo', req),
  resetPassword: (req: { ociCfgId: string; userId: string }) => post('/oci/tenant/resetPassword', req),
  deleteMfaDevice: (req: { ociCfgId: string; userId: string }) => post('/oci/tenant/deleteMfaDevice', req),
  deleteApiKey: (req: { ociCfgId: string; userId: string }) => post('/oci/tenant/deleteApiKey', req),
  deleteUser: (req: { ociCfgId: string; userId: string }) => post('/oci/tenant/deleteUser', req),

  images: (req: { configId: string; region: string; architecture: string; shape?: string }) =>
    post<ImageInfo[]>('/oci/images', req),
  shapes: (req: { configId: string; region: string; clearCache?: boolean }) => post<ShapeInfo[]>('/oci/shapes', req),

  trafficCondition: (configId: string) => get<TrafficCondition>('/oci/traffic/condition', { params: { configId } }),
  trafficVnics: (req: { configId: string; instanceId: string }) =>
    get<ValueLabel[]>('/oci/traffic/vnics', { params: req }),
  trafficData: (req: { configId: string; instanceId: string; vnicId: string; startTime: string; endTime: string }) =>
    post<TrafficData>('/oci/traffic/data', req),

  elbList: (configId: string) => post<ElbPoolItem[]>('/oci/elb/list', { configId }),
  elbCreate: (req: { configId: string; vcnId: string; subnetId?: string; nlbName?: string }) =>
    post<ElbPoolItem>('/oci/elb/create', req),
  elbDelete: (req: { configId: string; nlbId: string }) => post('/oci/elb/delete', req)
}

/** VCN 安全规则相关接口（独立分组）。 */
export const vcnApi = {
  securityList: (req: { configId: string; vcnId: string }) => post<SecurityListData>('/oci/vcn/securityList', req),
  addSecurityRule: (req: AddSecurityRuleReq) => post('/oci/vcn/addSecurityRule', req),
  releaseSecurityRules: (req: { configId: string; vcnId?: string }) => post('/oci/vcn/releaseSecurityRules', req),
  delete: (req: { configId: string; vcnId?: string }) => post('/oci/vcn/delete', req),
  create: (req: { configId: string; displayName?: string; cidrBlock?: string; isIpv6Enabled?: boolean }) =>
    post<VCNInfo>('/oci/vcn/create', req)
}
