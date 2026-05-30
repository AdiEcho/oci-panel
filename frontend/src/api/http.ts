import api from '@/lib/api'
import type { AxiosRequestConfig } from 'axios'

/**
 * 后端统一响应信封。
 *
 * 注意：`src/lib/api.ts` 的响应拦截器已把 axios 的 `AxiosResponse` 解包为后端 body
 * （即 `{ code, message, data }`），因此运行时 `api.post/api.get` 实际 resolve 为本类型；
 * 但 axios 的静态类型仍认为返回 `AxiosResponse`，故此处通过 `as unknown as` 补回正确类型。
 * 这样调用方拿到的 `response.data` 即业务数据，与改造前的写法完全一致（只是多了类型）。
 */
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

/** 分页列表响应的通用形状（userPage / task list / key list 等）。 */
export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  pageSize: number
}

/** 下拉选项通用 `{ value, label }`（流量查询的区域/实例/VNIC 选项等）。 */
export interface ValueLabel {
  value: string
  label: string
}

/** 类型化 POST：运行时等价于 `api.post`，返回值类型修正为已解包的 `ApiResponse<T>`。 */
export function post<T = unknown>(url: string, body?: unknown, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
  return api.post(url, body, config) as unknown as Promise<ApiResponse<T>>
}

/** 类型化 GET：运行时等价于 `api.get`，返回值类型修正为已解包的 `ApiResponse<T>`。 */
export function get<T = unknown>(url: string, config?: AxiosRequestConfig): Promise<ApiResponse<T>> {
  return api.get(url, config) as unknown as Promise<ApiResponse<T>>
}
