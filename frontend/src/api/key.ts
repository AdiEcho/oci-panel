import { get, post, type PageResult } from './http'

/** SSH 密钥列表项。 */
export interface KeyItem {
  id: number
  name: string
  keyType: string
  publicKey: string
  configName?: string
  createTime: string
}

/** 密钥列表分页查询参数。 */
export interface KeyListReq {
  page: number
  pageSize: number
  name?: string
  keyType?: string
}

export const keyApi = {
  list: (req: KeyListReq) => get<PageResult<KeyItem>>('/key/list', { params: req }),
  // standalone 公钥下拉：Configs 期望 id 为 number、Presets 期望 id 为 string，
  // 两调用方对同一接口的 id 类型期望冲突，故保留 any 以零行为回归（等价于原 api.get）。
  standalone: () => get<any[]>('/key/standalone'),
  create: (req: { name: string; publicKey: string }) => post('/key/create', req),
  update: (req: { id: number; name: string; publicKey: string }) => post('/key/update', req),
  delete: (ids: number[]) => post('/key/delete', { ids })
}
