<template>
  <div class="space-y-6">
    <div class="flex justify-between items-center flex-wrap gap-4">
      <h1 class="text-3xl font-bold">实例管理</h1>
      <div class="flex gap-3">
        <select v-model="selectedUserId" @change="loadInstances" class="input w-64">
          <option value="">选择用户</option>
          <option v-for="user in userList" :key="user.id" :value="user.id">
            {{ user.username }}
          </option>
        </select>
        <button @click="loadInstances" class="btn btn-primary" :disabled="!selectedUserId">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          刷新
        </button>
      </div>
    </div>

    <div v-if="loading" class="text-center py-12">
      <svg class="animate-spin h-12 w-12 mx-auto text-blue-500" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
    </div>

    <div v-else-if="!instances.length" class="card p-12 text-center">
      <svg class="w-16 h-16 mx-auto text-slate-600 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
      </svg>
      <p class="text-slate-400">{{ selectedUserId ? '暂无实例' : '请选择用户查看实例' }}</p>
    </div>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
      <div v-for="instance in instances" :key="instance.id" class="card p-6 hover:border-blue-500/50 transition-all">
        <div class="flex justify-between items-start mb-4">
          <div class="flex-1 min-w-0">
            <h3 class="text-lg font-semibold truncate">{{ instance.displayName || '未命名' }}</h3>
            <p class="text-xs text-slate-400 font-mono truncate">{{ instance.id }}</p>
          </div>
          <span 
            class="badge ml-2 flex-shrink-0"
            :class="{
              'badge-success': instance.lifecycleState === 'RUNNING',
              'badge-danger': instance.lifecycleState === 'STOPPED',
              'badge-warning': !['RUNNING', 'STOPPED'].includes(instance.lifecycleState)
            }"
          >
            {{ instance.lifecycleState }}
          </span>
        </div>

        <div class="space-y-2 mb-4 text-sm">
          <div class="flex justify-between">
            <span class="text-slate-400">区域</span>
            <span>{{ instance.region || 'N/A' }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-400">形状</span>
            <span>{{ instance.shape || 'N/A' }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-slate-400">公网IP</span>
            <span class="font-mono text-xs">{{ instance.publicIp || '无' }}</span>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-2">
          <button 
            @click="controlInstance(instance.id, 'start')" 
            class="btn btn-success text-sm"
            :disabled="instance.lifecycleState === 'RUNNING' || actionLoading[instance.id]"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            启动
          </button>
          <button 
            @click="controlInstance(instance.id, 'stop')" 
            class="btn btn-warning text-sm"
            :disabled="instance.lifecycleState !== 'RUNNING' || actionLoading[instance.id]"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
            </svg>
            停止
          </button>
          <button 
            @click="controlInstance(instance.id, 'reboot')" 
            class="btn btn-secondary text-sm"
            :disabled="instance.lifecycleState !== 'RUNNING' || actionLoading[instance.id]"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            重启
          </button>
          <button 
            @click="controlInstance(instance.id, 'terminate')" 
            class="btn btn-danger text-sm"
            :disabled="actionLoading[instance.id]"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            删除
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import api from '../utils/api'
import { toast } from '../utils/toast'

const userList = ref([])
const selectedUserId = ref('')
const instances = ref([])
const loading = ref(false)
const actionLoading = reactive({})

const loadUserList = async () => {
  try {
    const response = await api.post('/oci/userPage', { page: 1, pageSize: 100 })
    userList.value = response.data.list || []
  } catch (error) {
    console.error('加载用户列表失败:', error)
  }
}

const loadInstances = async () => {
  if (!selectedUserId.value) {
    toast.warning('请先选择用户')
    return
  }

  loading.value = true
  try {
    const user = userList.value.find(u => u.id === selectedUserId.value)
    const response = await api.post('/instance/list', {
      userId: selectedUserId.value,
      compartmentId: user?.ociTenantId || ''
    })
    instances.value = response.data || []
  } catch (error) {
    toast.error(error.message || '加载实例失败')
    instances.value = []
  } finally {
    loading.value = false
  }
}

const controlInstance = async (instanceId, action) => {
  const actionMap = {
    start: { endpoint: '/instance/start', message: '启动' },
    stop: { endpoint: '/instance/stop', message: '停止' },
    reboot: { endpoint: '/instance/reboot', message: '重启' },
    terminate: { endpoint: '/instance/terminate', message: '删除' }
  }

  if (action === 'terminate' && !confirm('确定要删除此实例吗？此操作不可恢复！')) {
    return
  }

  actionLoading[instanceId] = true
  try {
    await api.post(actionMap[action].endpoint, {
      userId: selectedUserId.value,
      instanceId
    })
    toast.success(`${actionMap[action].message}操作已提交`)
    setTimeout(() => loadInstances(), 2000)
  } catch (error) {
    toast.error(error.message || `${actionMap[action].message}失败`)
  } finally {
    delete actionLoading[instanceId]
  }
}

onMounted(() => {
  loadUserList()
})
</script>
