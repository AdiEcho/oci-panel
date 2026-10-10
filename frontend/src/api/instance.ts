import { post } from './http'
import type { Bandwidth500Endpoint, Bandwidth500Summary } from './oci'

export const instanceApi = {
  start: (userId: string, instanceId: string) => post('/instance/start', { userId, instanceId }),
  stop: (userId: string, instanceId: string) => post('/instance/stop', { userId, instanceId }),
  reboot: (userId: string, instanceId: string) => post('/instance/reboot', { userId, instanceId }),
  terminate: (userId: string, instanceId: string) => post('/instance/terminate', { userId, instanceId }),
  changeIP: (userId: string, instanceId: string) =>
    post<{ newIP?: string }>('/instance/changeIP', { userId, instanceId }),
  updateName: (req: { userId: string; instanceId?: string; displayName: string }) => post('/instance/updateName', req),
  updateConfig: (req: { userId: string; instanceId?: string; ocpus: number; memoryInGBs: number }) =>
    post('/instance/updateConfig', req),
  updateBootVolume: (req: { userId: string; instanceId?: string; sizeInGBs: number; vpusPerGB: number }) =>
    post('/instance/updateBootVolume', req),
  attachIPv6: (req: { userId: string; instanceId?: string }) => post<{ ipv6?: string }>('/instance/attachIPv6', req),
  autoRescue: (req: { userId: string; instanceId?: string; instanceName: string; keepBackup: boolean }) =>
    post('/instance/autoRescue', req),
  enable500Mbps: (req: {
    userId: string
    instanceId?: string
    sshPort?: number
    mode?: string
    mappings?: Bandwidth500Endpoint[]
    nlbId?: string
  }) => post('/instance/enable500Mbps', req),
  disable500Mbps: (req: { userId: string; instanceId?: string; retainNatGw?: boolean; retainNlb?: boolean }) =>
    post('/instance/disable500Mbps', req),
  getStatus500Mbps: (userId: string, instanceId: string) =>
    post<Bandwidth500Summary>('/instance/bandwidth500/status', { userId, instanceId }),
  plan500Mbps: (req: {
    userId: string
    instanceId: string
    mode?: string
    sshPort?: number
    mappings?: Bandwidth500Endpoint[]
    nlbId?: string
  }) => post<any>('/instance/bandwidth500/plan', req),
  migrate500Mbps: (req: { userId: string; instanceId: string; sshPort?: number; mappings?: Bandwidth500Endpoint[] }) =>
    post('/instance/bandwidth500/migrate', req),
  createCloudShell: (req: { userId: string; instanceId: string; publicKey: string }) =>
    post<{ connectionId?: string; connectionString?: string }>('/instance/createCloudShell', req)
}
