import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '@/lib/api'

export interface User {
  username: string
  account: string
}

export interface LoginResult {
  needMfa: boolean
  needPasskey: boolean
  passkeyEnabled: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem('token') || '')
  const user = ref<User | null>(JSON.parse(localStorage.getItem('user') || 'null'))
  const pendingAccount = ref<string>('')

  const isAuthenticated = computed(() => !!token.value)

  async function login(account: string, password: string): Promise<LoginResult> {
    const response = await api.post('/sys/login', { account, password })

    if (response.data.needMfa || response.data.needPasskey) {
      pendingAccount.value = account
      return {
        needMfa: response.data.needMfa || false,
        needPasskey: response.data.needPasskey || false,
        passkeyEnabled: response.data.passkeyEnabled || false
      }
    }

    token.value = response.data.token
    user.value = {
      username: response.data.username,
      account: account
    }

    localStorage.setItem('token', token.value)
    localStorage.setItem('user', JSON.stringify(user.value))

    return { needMfa: false, needPasskey: false, passkeyEnabled: false }
  }

  function setToken(newToken: string, username: string): void {
    token.value = newToken
    user.value = {
      username: username,
      account: pendingAccount.value || username
    }
    localStorage.setItem('token', token.value)
    localStorage.setItem('user', JSON.stringify(user.value))
    pendingAccount.value = ''
  }

  async function verifyMfa(code: string): Promise<void> {
    const response = await api.post('/sys/checkMfaCode', { code })

    token.value = response.data.token
    user.value = {
      username: response.data.username,
      account: pendingAccount.value
    }

    localStorage.setItem('token', token.value)
    localStorage.setItem('user', JSON.stringify(user.value))
    pendingAccount.value = ''
  }

  function logout() {
    token.value = ''
    user.value = null
    pendingAccount.value = ''
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  function checkAuth() {
    if (!token.value) {
      logout()
    }
  }

  return {
    token,
    user,
    isAuthenticated,
    login,
    setToken,
    verifyMfa,
    logout,
    checkAuth
  }
})
