import { post, type PageResult } from './http'

/** 实例创建任务列表项。 */
export interface TaskItem {
  id: number
  username: string
  ociRegion: string
  architecture: string
  operationSystem: string
  ocpus: number
  memory: number
  disk: number
  interval: number
  status: string
  executeCount: number
  successCount: number
  lastExecuteTime?: string
  lastMessage?: string
}

/** 任务执行日志。 */
export interface TaskLog {
  id: number
  status: string
  message: string
  executeTime: string
}

/** 创建实例任务的入参（userId 来自配置 id，可能为 number/undefined）。 */
export interface CreateTaskReq {
  userId: number | string | undefined
  ociRegion: string
  ocpus: number
  memory: number
  disk: number
  bootVolumeVpu?: number
  architecture: string
  operationSystem: string
  imageId?: string
  sshKeyId: string
  interval: number
  executeOnce?: boolean
}

export const taskApi = {
  create: (req: CreateTaskReq) => post('/task/create', req),
  list: (req: { page: number; pageSize: number; status?: string }) => post<PageResult<TaskItem>>('/task/list', req),
  start: (taskId: number) => post('/task/start', { taskId }),
  stop: (taskId: number) => post('/task/stop', { taskId }),
  delete: (taskId: number) => post('/task/delete', { taskId }),
  batchDelete: (taskIds: number[]) => post('/task/batchDelete', { taskIds }),
  logs: (req: { taskId: number; page: number; pageSize: number }) => post<PageResult<TaskLog>>('/task/logs', req),
  clearLogs: (taskId: number) => post('/task/clearLogs', { taskId })
}
