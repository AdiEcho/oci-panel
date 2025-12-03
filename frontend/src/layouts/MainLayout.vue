<template>
  <div class="flex h-screen overflow-hidden">
    <!-- Sidebar -->
    <aside
      class="w-64 bg-slate-800/50 backdrop-blur-sm border-r border-slate-700 flex-shrink-0 transition-all duration-300"
      :class="{ '-ml-64': !sidebarOpen }"
    >
      <div class="h-full flex flex-col">
        <!-- Logo -->
        <div class="p-6 border-b border-slate-700">
          <div class="flex items-center gap-3">
            <div
              class="w-10 h-10 bg-gradient-to-br from-blue-500 to-purple-600 rounded-xl flex items-center justify-center"
            >
              <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z"
                />
              </svg>
            </div>
            <div>
              <h1 class="text-xl font-bold bg-gradient-to-r from-blue-400 to-purple-400 bg-clip-text text-transparent">
                OCI Panel
              </h1>
            </div>
          </div>
        </div>

        <!-- Navigation -->
        <nav class="flex-1 p-4 space-y-2 overflow-y-auto">
          <router-link
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            class="flex items-center gap-3 px-4 py-3 rounded-lg transition-all duration-200 hover:bg-slate-700/50"
            :class="
              $route.path === item.path
                ? 'bg-gradient-to-r from-blue-500/20 to-purple-600/20 text-blue-400 border border-blue-500/30'
                : 'text-slate-300'
            "
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" v-html="item.icon"></svg>
            <span class="font-medium">{{ item.label }}</span>
          </router-link>
        </nav>

        <!-- User Info -->
        <div class="p-4 border-t border-slate-700">
          <div class="flex items-center gap-3 mb-3 px-2">
            <div
              class="w-10 h-10 bg-gradient-to-br from-green-500 to-teal-600 rounded-full flex items-center justify-center"
            >
              <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-white truncate">{{ authStore.user?.username || '管理员' }}</p>
              <p class="text-xs text-slate-400">系统管理员</p>
            </div>
          </div>
          <button class="btn btn-secondary w-full" @click="handleLogout">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
              />
            </svg>
            退出登录
          </button>
        </div>
      </div>
    </aside>

    <!-- Main Content -->
    <div class="flex-1 flex flex-col overflow-hidden">
      <!-- Topbar -->
      <header
        class="h-16 bg-slate-800/50 backdrop-blur-sm border-b border-slate-700 flex items-center justify-between px-6"
      >
        <button class="lg:hidden p-2 hover:bg-slate-700 rounded-lg" @click="sidebarOpen = !sidebarOpen">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>

        <div class="flex-1"></div>

        <div class="flex items-center gap-4">
          <div class="text-sm text-slate-400">
            {{ currentTime }}
          </div>
        </div>
      </header>

      <!-- Page Content -->
      <main class="flex-1 overflow-y-auto p-6">
        <router-view />
      </main>
    </div>

    <!-- Toast Notifications -->
    <div class="fixed top-4 right-4 z-50 space-y-2">
      <transition-group name="toast">
        <div
          v-for="t in toast.toasts.value"
          :key="t.id"
          class="card p-4 min-w-[300px] flex items-start gap-3 animate-slide-in-right"
          :class="{
            'border-green-500/50': t.type === 'success',
            'border-red-500/50': t.type === 'error',
            'border-yellow-500/50': t.type === 'warning',
            'border-blue-500/50': t.type === 'info'
          }"
        >
          <div class="flex-shrink-0">
            <svg v-if="t.type === 'success'" class="w-5 h-5 text-green-400" fill="currentColor" viewBox="0 0 20 20">
              <path
                fill-rule="evenodd"
                d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                clip-rule="evenodd"
              />
            </svg>
            <svg v-else-if="t.type === 'error'" class="w-5 h-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
              <path
                fill-rule="evenodd"
                d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                clip-rule="evenodd"
              />
            </svg>
            <svg
              v-else-if="t.type === 'warning'"
              class="w-5 h-5 text-yellow-400"
              fill="currentColor"
              viewBox="0 0 20 20"
            >
              <path
                fill-rule="evenodd"
                d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
                clip-rule="evenodd"
              />
            </svg>
            <svg v-else class="w-5 h-5 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
              <path
                fill-rule="evenodd"
                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"
                clip-rule="evenodd"
              />
            </svg>
          </div>
          <div class="flex-1">
            <p class="text-sm text-white">{{ t.message }}</p>
          </div>
          <button class="flex-shrink-0 text-slate-400 hover:text-white" @click="toast.remove(t.id)">
            <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
              <path
                fill-rule="evenodd"
                d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                clip-rule="evenodd"
              />
            </svg>
          </button>
        </div>
      </transition-group>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { toast } from '../utils/toast'

const router = useRouter()
const authStore = useAuthStore()
const sidebarOpen = ref(true)
const currentTime = ref('')

const navItems = [
  {
    path: '/',
    label: '概览',
    icon: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />'
  },
  {
    path: '/configs',
    label: '配置管理',
    icon: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />'
  },
  {
    path: '/keys',
    label: '密钥管理',
    icon: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />'
  },
  {
    path: '/logs',
    label: '实时日志',
    icon: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />'
  },
  {
    path: '/settings',
    label: '系统设置',
    icon: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />'
  }
]

const updateTime = () => {
  const now = new Date()
  currentTime.value = now.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

let timer
onMounted(() => {
  updateTime()
  timer = setInterval(updateTime, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const handleLogout = () => {
  authStore.logout()
  toast.info('已退出登录')
  router.push('/login')
}
</script>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translateX(100%);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(100%);
}
</style>
