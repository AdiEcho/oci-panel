<template>
  <div class="space-y-6">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl font-bold">实时日志</h1>
      <div class="flex gap-3">
        <button class="btn btn-secondary" @click="clearLogs">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
            />
          </svg>
          清空日志
        </button>
        <button :class="isConnected ? 'btn-danger' : 'btn-primary'" class="btn" @click="toggleConnection">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              v-if="isConnected"
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M18.364 5.636a9 9 0 010 12.728m0 0l-2.829-2.829m2.829 2.829L21 21M15.536 8.464a5 5 0 010 7.072m0 0l-2.829-2.829m-4.243 2.829a4.978 4.978 0 01-1.414-2.83m-1.414 5.658a9 9 0 01-2.167-9.238m7.824 2.167a1 1 0 111.414 1.414m-1.414-1.414L3 3m8.293 8.293l1.414 1.414"
            />
            <path
              v-else
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0"
            />
          </svg>
          {{ isConnected ? '断开连接' : '连接' }}
        </button>
      </div>
    </div>

    <div class="card">
      <div class="p-6">
        <div ref="logConsole" class="bg-black rounded-lg p-4 h-[600px] overflow-y-auto font-mono text-sm">
          <div
            v-for="(log, index) in logs"
            :key="index"
            class="mb-1"
            :class="{
              'text-green-400': log.type === 'info',
              'text-red-400': log.type === 'error',
              'text-yellow-400': log.type === 'warning',
              'text-blue-400': log.type === 'success'
            }"
          >
            {{ log.message }}
          </div>
          <div v-if="!logs.length" class="text-slate-500 text-center py-8">
            {{ isConnected ? '等待日志输出...' : '未连接，请点击"连接"按钮' }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onUnmounted, nextTick } from 'vue'
import { toast } from '../utils/toast'

const logs = ref([])
const isConnected = ref(false)
const ws = ref(null)
const logConsole = ref(null)

const addLog = (message, type = 'info') => {
  const timestamp = new Date().toLocaleTimeString('zh-CN')
  logs.value.push({
    message: `[${timestamp}] ${message}`,
    type
  })

  nextTick(() => {
    if (logConsole.value) {
      logConsole.value.scrollTop = logConsole.value.scrollHeight
    }
  })
}

const connectWebSocket = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws/logs`

  try {
    ws.value = new WebSocket(wsUrl)

    ws.value.onopen = () => {
      isConnected.value = true
      addLog('WebSocket 连接成功', 'success')
      toast.success('日志连接成功')
    }

    ws.value.onmessage = (event) => {
      addLog(event.data, 'info')
    }

    ws.value.onerror = (error) => {
      console.error('WebSocket错误:', error)
      addLog('WebSocket 连接错误', 'error')
      toast.error('WebSocket连接错误')
    }

    ws.value.onclose = () => {
      isConnected.value = false
      addLog('WebSocket 连接已断开', 'warning')
    }
  } catch (error) {
    console.error('创建WebSocket失败:', error)
    toast.error('无法建立WebSocket连接')
  }
}

const disconnectWebSocket = () => {
  if (ws.value) {
    ws.value.close()
    ws.value = null
  }
}

const toggleConnection = () => {
  if (isConnected.value) {
    disconnectWebSocket()
    toast.info('已断开连接')
  } else {
    connectWebSocket()
  }
}

const clearLogs = () => {
  logs.value = []
  toast.info('日志已清空')
}

onUnmounted(() => {
  disconnectWebSocket()
})
</script>
