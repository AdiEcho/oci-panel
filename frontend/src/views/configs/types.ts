// 配置管理相关共享类型（C7 拆分后由 Configs.vue / ConfigDetailsDrawer / InstancesTab 复用）。
// 引导卷 / VCN / 租户 / 流量等结构后端字段繁多且部分动态，沿用既有 `any`（与拆分前一致）。

export interface Config {
  id: number
  username: string
  tenantName?: string
  tenantCreateTime?: string
  ociRegion: string
  ociUserId?: string
  ociFingerprint?: string
  ociTenantId?: string
  ociKeyPath?: string
  instanceCount?: number
  runningInstances?: number
}

export interface Instance {
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
}
