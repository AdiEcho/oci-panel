import { get, post } from './http'

/** 系统配置（密钥目录 / 日志级别 / 缓存开关）。 */
export interface SysConfig {
  keyDirPath: string
  logLevel: string
  cacheEnabled?: boolean
  cacheInterval?: number
}

/** 登录结果：未完成时返回需要的二次验证标志，完成时带 token。 */
export interface LoginResultData {
  needMfa?: boolean
  needPasskey?: boolean
  passkeyEnabled?: boolean
  token?: string
  username?: string
}

/** 颁发 token 的结果（MFA / Passkey 验证成功后）。 */
export interface TokenData {
  token: string
  username: string
}

/** 认证状态（MFA / Passkey 是否启用）。 */
export interface AuthStatus {
  mfaEnabled: boolean
  passkeyEnabled: boolean
}

/** 生成 MFA 密钥的结果。 */
export interface MfaSecret {
  secret: string
  qrCode: string
}

/** 概览统计。 */
export interface Glance {
  totalConfigs: number
  totalTasks: number
}

export const sysApi = {
  login: (account: string, password: string) => post<LoginResultData>('/sys/login', { account, password }),
  checkMfaCode: (code: string) => post<TokenData>('/sys/checkMfaCode', { code }),
  getSysCfg: () => get<SysConfig>('/sys/getSysCfg'),
  updateCacheCfg: (req: { cacheEnabled: boolean; cacheInterval: number }) => post('/sys/updateCacheCfg', req),
  refreshCache: () => post('/sys/refreshCache', {}),
  getAuthStatus: () => get<AuthStatus>('/sys/getAuthStatus'),
  generateMfaSecret: () => post<MfaSecret>('/sys/generateMfaSecret', {}),
  enableMfa: (req: { secret: string; code: string }) => post('/sys/enableMfa', req),
  disableMfa: () => post('/sys/disableMfa', {}),
  getGlance: () => get<Glance>('/sys/getGlance')
}
