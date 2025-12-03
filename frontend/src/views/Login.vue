<template>
  <div
    class="min-h-screen flex items-center justify-center p-4 bg-gradient-to-br from-blue-600 via-purple-600 to-pink-600"
  >
    <div class="w-full max-w-md animate-slide-in-up">
      <div class="card p-8">
        <div class="text-center mb-8">
          <div
            class="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-blue-500 to-purple-600 rounded-2xl mb-4"
          >
            <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z"
              />
            </svg>
          </div>
          <h1 class="text-3xl font-bold bg-gradient-to-r from-blue-400 to-purple-400 bg-clip-text text-transparent">
            OCI Panel
          </h1>
          <p class="text-slate-400 mt-2">Oracle Cloud Infrastructure 管理面板</p>
        </div>

        <form class="space-y-6" @submit.prevent="handleLogin">
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">
              <svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                />
              </svg>
              账号
            </label>
            <input
              v-model="form.account"
              type="text"
              class="input"
              placeholder="请输入账号"
              required
              autocomplete="username"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">
              <svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                />
              </svg>
              密码
            </label>
            <input
              v-model="form.password"
              type="password"
              class="input"
              placeholder="请输入密码"
              required
              autocomplete="current-password"
            />
          </div>

          <button type="submit" class="btn btn-primary w-full py-3 text-lg" :disabled="loading">
            <svg v-if="loading" class="animate-spin w-5 h-5" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            <span v-else>登录</span>
          </button>
        </form>

        <div v-if="error" class="mt-4 p-3 bg-red-500/20 border border-red-500/50 rounded-lg text-red-400 text-sm">
          {{ error }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { toast } from '../utils/toast'

const router = useRouter()
const authStore = useAuthStore()

const form = ref({
  account: '',
  password: ''
})

const loading = ref(false)
const error = ref('')

const handleLogin = async () => {
  error.value = ''
  loading.value = true

  try {
    await authStore.login(form.value.account, form.value.password)
    toast.success('登录成功')
    router.push('/')
  } catch (err) {
    error.value = err.message || '登录失败，请检查账号密码'
    toast.error(error.value)
  } finally {
    loading.value = false
  }
}
</script>
