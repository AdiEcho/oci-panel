import { get, post, type PageResult } from './http'

/** 代理协议。 */
export type ProxyProtocol = 'http' | 'https' | 'socks5'

/** 代理列表项（响应不回传密码）。 */
export interface ProxyItem {
  id: string
  name: string
  protocol: ProxyProtocol
  host: string
  port: number
  username: string
  hasAuth: boolean
  enabled: boolean
  remark: string
  bindCount: number
  createTime: string
}

export interface ProxyListReq {
  page: number
  pageSize: number
  name?: string
  protocol?: string
}

export interface ProxySaveReq {
  id?: string
  name: string
  protocol: string
  host: string
  port: number
  username?: string
  password?: string
  enabled?: boolean
  remark?: string
}

export interface ProxyTestReq {
  id?: string
  protocol?: string
  host?: string
  port?: number
  username?: string
  password?: string
  target?: string
}

export interface ProxyTestResult {
  reachable: boolean
  latencyMs: number
  egressIp: string
  message: string
}

export const proxyApi = {
  page: (req: ProxyListReq) => post<PageResult<ProxyItem>>('/proxy/page', req),
  list: () => get<ProxyItem[]>('/proxy/list'),
  create: (req: ProxySaveReq) => post<ProxyItem>('/proxy/create', req),
  update: (req: ProxySaveReq) => post<ProxyItem>('/proxy/update', req),
  delete: (ids: string[]) => post('/proxy/delete', { ids }),
  test: (req: ProxyTestReq) => post<ProxyTestResult>('/proxy/test', req),
  batchBind: (configIds: string[], proxyId: string) =>
    post<{ updated: number }>('/proxy/batchBind', { configIds, proxyId })
}
