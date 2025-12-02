<template>
  <div class="space-y-6">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl font-bold">用户管理</h1>
      <button @click="showAddModal = true" class="btn btn-primary">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        添加配置
      </button>
    </div>

    <div class="card">
      <div class="p-6 border-b border-slate-700">
        <input
          v-model="searchText"
          @input="handleSearch"
          type="text"
          placeholder="搜索用户名..."
          class="input max-w-md"
        />
      </div>

      <div class="overflow-x-auto">
        <table class="w-full">
          <thead class="bg-slate-700/50">
            <tr>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">用户名</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">租户名称</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">区域</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">创建时间</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-700">
            <tr v-if="loading">
              <td colspan="5" class="px-6 py-8 text-center text-slate-400">
                <svg class="animate-spin h-8 w-8 mx-auto" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              </td>
            </tr>
            <tr v-else-if="!users.length">
              <td colspan="5" class="px-6 py-8 text-center text-slate-400">暂无数据</td>
            </tr>
            <tr v-else v-for="user in users" :key="user.id" class="hover:bg-slate-700/30">
              <td class="px-6 py-4">{{ user.username }}</td>
              <td class="px-6 py-4">{{ user.tenantName }}</td>
              <td class="px-6 py-4">{{ user.ociRegion }}</td>
              <td class="px-6 py-4">{{ formatDate(user.createTime) }}</td>
              <td class="px-6 py-4">
                <div class="flex gap-2">
                  <button @click="editUser(user)" class="text-blue-400 hover:text-blue-300">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                    </svg>
                  </button>
                  <button @click="deleteUser(user.id)" class="text-red-400 hover:text-red-300">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="totalPages > 1" class="p-6 border-t border-slate-700 flex justify-center gap-2">
        <button
          @click="loadUsers(currentPage - 1)"
          :disabled="currentPage === 1"
          class="btn btn-secondary"
        >
          上一页
        </button>
        <span class="flex items-center px-4 text-slate-300">
          第 {{ currentPage }} / {{ totalPages }} 页
        </span>
        <button
          @click="loadUsers(currentPage + 1)"
          :disabled="currentPage === totalPages"
          class="btn btn-secondary"
        >
          下一页
        </button>
      </div>
    </div>

    <!-- Add/Edit Modal -->
    <div v-if="showAddModal" class="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div class="card max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        <div class="p-6 border-b border-slate-700 flex justify-between items-center">
          <h3 class="text-xl font-bold">{{ editingUser ? '编辑配置' : '添加OCI配置' }}</h3>
          <button @click="closeModal" class="text-slate-400 hover:text-white">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <form @submit.prevent="submitForm" class="p-6 space-y-4">
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">用户名</label>
            <input v-model="form.username" type="text" class="input" required />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">租户名称</label>
            <input v-model="form.tenantName" type="text" class="input" required />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">Tenant ID</label>
            <input v-model="form.ociTenantId" type="text" class="input" required />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">User ID</label>
            <input v-model="form.ociUserId" type="text" class="input" required />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">Fingerprint</label>
            <input v-model="form.ociFingerprint" type="text" class="input" required />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">Region</label>
            <input v-model="form.ociRegion" type="text" class="input" placeholder="例: ap-singapore-1" required />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">Key Path</label>
            <input v-model="form.ociKeyPath" type="text" class="input" placeholder="例: /keys/oci_api_key.pem" required />
          </div>

          <div class="flex gap-3 pt-4">
            <button type="button" @click="closeModal" class="btn btn-secondary flex-1">取消</button>
            <button type="submit" class="btn btn-primary flex-1" :disabled="submitting">
              {{ submitting ? '提交中...' : '提交' }}
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

const users = ref([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = 10
const totalPages = ref(0)
const searchText = ref('')
const showAddModal = ref(false)
const editingUser = ref(null)
const submitting = ref(false)

const form = ref({
  username: '',
  tenantName: '',
  ociTenantId: '',
  ociUserId: '',
  ociFingerprint: '',
  ociRegion: '',
  ociKeyPath: ''
})

const loadUsers = async (page = 1) => {
  loading.value = true
  try {
    const response = await api.post('/oci/userPage', {
      page,
      pageSize,
      username: searchText.value
    })
    users.value = response.data.list || []
    currentPage.value = response.data.page
    totalPages.value = Math.ceil(response.data.total / pageSize)
  } catch (error) {
    toast.error(error.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  loadUsers(1)
}

const closeModal = () => {
  showAddModal.value = false
  editingUser.value = null
  form.value = {
    username: '',
    tenantName: '',
    ociTenantId: '',
    ociUserId: '',
    ociFingerprint: '',
    ociRegion: '',
    ociKeyPath: ''
  }
}

const submitForm = async () => {
  submitting.value = true
  try {
    if (editingUser.value) {
      await api.post('/oci/updateCfgName', {
        id: editingUser.value.id,
        username: form.value.username
      })
      toast.success('更新成功')
    } else {
      await api.post('/oci/addCfg', form.value)
      toast.success('添加成功')
    }
    closeModal()
    await loadUsers(currentPage.value)
  } catch (error) {
    toast.error(error.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const editUser = (user) => {
  editingUser.value = user
  form.value = { ...user }
  showAddModal.value = true
}

const deleteUser = async (id) => {
  if (!confirm('确定要删除此配置吗？')) return
  
  try {
    await api.post('/oci/removeCfg', { ids: [id] })
    toast.success('删除成功')
    await loadUsers(currentPage.value)
  } catch (error) {
    toast.error(error.message || '删除失败')
  }
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
}

onMounted(() => {
  loadUsers()
})
</script>
