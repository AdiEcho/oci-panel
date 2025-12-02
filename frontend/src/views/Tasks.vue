<template>
  <div class="space-y-6">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl font-bold">创建任务</h1>
      <button @click="showCreateModal = true" class="btn btn-primary">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        新建任务
      </button>
    </div>

    <div class="card">
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead class="bg-slate-700/50">
            <tr>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">用户ID</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">区域</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">配置</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">架构</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">系统</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">创建时间</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-700">
            <tr v-if="loading">
              <td colspan="6" class="px-6 py-8 text-center">
                <svg class="animate-spin h-8 w-8 mx-auto" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              </td>
            </tr>
            <tr v-else-if="!tasks.length">
              <td colspan="6" class="px-6 py-8 text-center text-slate-400">暂无任务</td>
            </tr>
            <tr v-else v-for="task in tasks" :key="task.id" class="hover:bg-slate-700/30">
              <td class="px-6 py-4 font-mono text-xs">{{ task.userId?.substring(0, 15) }}...</td>
              <td class="px-6 py-4">{{ task.ociRegion }}</td>
              <td class="px-6 py-4">{{ task.ocpus }}C / {{ task.memory }}GB / {{ task.disk }}GB</td>
              <td class="px-6 py-4">
                <span class="badge" :class="task.architecture === 'ARM' ? 'badge-info' : 'badge-warning'">
                  {{ task.architecture }}
                </span>
              </td>
              <td class="px-6 py-4">{{ task.operationSystem }}</td>
              <td class="px-6 py-4">{{ formatDate(task.createTime) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="totalPages > 1" class="p-6 border-t border-slate-700 flex justify-center gap-2">
        <button @click="loadTasks(currentPage - 1)" :disabled="currentPage === 1" class="btn btn-secondary">上一页</button>
        <span class="flex items-center px-4">第 {{ currentPage }} / {{ totalPages }} 页</span>
        <button @click="loadTasks(currentPage + 1)" :disabled="currentPage === totalPages" class="btn btn-secondary">下一页</button>
      </div>
    </div>

    <!-- Create Task Modal -->
    <div v-if="showCreateModal" class="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div class="card max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        <div class="p-6 border-b border-slate-700 flex justify-between items-center">
          <h3 class="text-xl font-bold">创建实例任务</h3>
          <button @click="closeModal" class="text-slate-400 hover:text-white">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <form @submit.prevent="submitTask" class="p-6 space-y-4">
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">选择用户</label>
            <select v-model="form.userId" class="input" required>
              <option value="">请选择用户</option>
              <option v-for="user in userList" :key="user.id" :value="user.id">
                {{ user.username }}
              </option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">区域</label>
            <input v-model="form.ociRegion" type="text" class="input" placeholder="例: ap-singapore-1" required />
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-slate-300 mb-2">CPU核心数</label>
              <input v-model.number="form.ocpus" type="number" step="0.1" min="0.1" class="input" required />
            </div>
            <div>
              <label class="block text-sm font-medium text-slate-300 mb-2">内存(GB)</label>
              <input v-model.number="form.memory" type="number" step="0.1" min="0.1" class="input" required />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-slate-300 mb-2">磁盘(GB)</label>
              <input v-model.number="form.disk" type="number" min="50" class="input" required />
            </div>
            <div>
              <label class="block text-sm font-medium text-slate-300 mb-2">架构</label>
              <select v-model="form.architecture" class="input" required>
                <option value="ARM">ARM</option>
                <option value="AMD">AMD</option>
              </select>
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">操作系统</label>
            <select v-model="form.operationSystem" class="input" required>
              <option value="Ubuntu">Ubuntu</option>
              <option value="CentOS">CentOS</option>
              <option value="Oracle Linux">Oracle Linux</option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">Root密码</label>
            <input v-model="form.rootPassword" type="password" class="input" required />
          </div>

          <div class="flex gap-3 pt-4">
            <button type="button" @click="closeModal" class="btn btn-secondary flex-1">取消</button>
            <button type="submit" class="btn btn-primary flex-1" :disabled="submitting">
              {{ submitting ? '创建中...' : '创建' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../utils/api'
import { toast } from '../utils/toast'

const tasks = ref([])
const userList = ref([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = 10
const totalPages = ref(0)
const showCreateModal = ref(false)
const submitting = ref(false)

const form = ref({
  userId: '',
  ociRegion: '',
  ocpus: 1,
  memory: 6,
  disk: 50,
  architecture: 'ARM',
  operationSystem: 'Ubuntu',
  rootPassword: ''
})

const loadTasks = async (page = 1) => {
  loading.value = true
  try {
    const response = await api.post('/oci/createTaskPage', {
      page,
      pageSize
    })
    tasks.value = response.data.list || []
    currentPage.value = response.data.page
    totalPages.value = Math.ceil(response.data.total / pageSize)
  } catch (error) {
    toast.error(error.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const loadUserList = async () => {
  try {
    const response = await api.post('/oci/userPage', { page: 1, pageSize: 100 })
    userList.value = response.data.list || []
  } catch (error) {
    console.error('加载用户列表失败:', error)
  }
}

const closeModal = () => {
  showCreateModal.value = false
  form.value = {
    userId: '',
    ociRegion: '',
    ocpus: 1,
    memory: 6,
    disk: 50,
    architecture: 'ARM',
    operationSystem: 'Ubuntu',
    rootPassword: ''
  }
}

const submitTask = async () => {
  submitting.value = true
  try {
    await api.post('/oci/createInstance', form.value)
    toast.success('任务创建成功')
    closeModal()
    await loadTasks(currentPage.value)
  } catch (error) {
    toast.error(error.message || '创建失败')
  } finally {
    submitting.value = false
  }
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
}

onMounted(() => {
  loadTasks()
  loadUserList()
})
</script>
