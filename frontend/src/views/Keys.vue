<template>
  <div class="space-y-6">
    <div class="flex justify-between items-center">
      <h1 class="text-3xl font-bold">密钥管理</h1>
      <button class="btn btn-primary" @click="showAddModal = true">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        添加公钥
      </button>
    </div>

    <div class="card">
      <div class="p-6 border-b border-slate-700 flex gap-4">
        <input
          v-model="searchText"
          type="text"
          placeholder="搜索密钥名..."
          class="input max-w-md"
          @input="handleSearch"
        />
        <select v-model="filterType" class="input max-w-xs" @change="loadKeys(1)">
          <option value="">全部类型</option>
          <option value="standalone">独立上传</option>
          <option value="config">配置关联</option>
        </select>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full">
          <thead class="bg-slate-700/50">
            <tr>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">名称</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">类型</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">公钥</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">关联配置</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">创建时间</th>
              <th class="px-6 py-4 text-left text-xs font-semibold text-slate-300 uppercase">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-700">
            <tr v-if="loading">
              <td colspan="6" class="px-6 py-8 text-center text-slate-400">
                <svg class="animate-spin h-8 w-8 mx-auto" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              </td>
            </tr>
            <tr v-else-if="!keys.length">
              <td colspan="6" class="px-6 py-8 text-center text-slate-400">暂无密钥</td>
            </tr>
            <tr v-for="key in keys" v-else :key="key.id" class="hover:bg-slate-700/30">
              <td class="px-6 py-4 font-medium">{{ key.name }}</td>
              <td class="px-6 py-4">
                <span
                  class="px-2 py-1 text-xs font-semibold rounded-full"
                  :class="key.keyType === 'standalone' ? 'bg-blue-500/20 text-blue-300' : 'bg-purple-500/20 text-purple-300'"
                >
                  {{ key.keyType === 'standalone' ? '独立上传' : '配置关联' }}
                </span>
              </td>
              <td class="px-6 py-4">
                <div class="flex items-center gap-2">
                  <span class="font-mono text-xs text-slate-400 truncate max-w-xs">{{ key.publicKey.substring(0, 50) }}...</span>
                  <button class="text-blue-400 hover:text-blue-300" title="复制公钥" @click="copyToClipboard(key.publicKey)">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                    </svg>
                  </button>
                </div>
              </td>
              <td class="px-6 py-4 text-sm text-slate-400">{{ key.configName || '-' }}</td>
              <td class="px-6 py-4 text-sm text-slate-400">{{ key.createTime }}</td>
              <td class="px-6 py-4">
                <div class="flex gap-2">
                  <button
                    v-if="key.keyType === 'standalone'"
                    class="text-blue-400 hover:text-blue-300"
                    title="编辑"
                    @click="editKey(key)"
                  >
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                    </svg>
                  </button>
                  <button
                    v-if="key.keyType === 'standalone'"
                    class="text-red-400 hover:text-red-300"
                    title="删除"
                    @click="deleteKey(key.id)"
                  >
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
        <button :disabled="currentPage === 1" class="btn btn-secondary" @click="loadKeys(currentPage - 1)">上一页</button>
        <span class="flex items-center px-4 text-slate-300">第 {{ currentPage }} / {{ totalPages }} 页</span>
        <button :disabled="currentPage === totalPages" class="btn btn-secondary" @click="loadKeys(currentPage + 1)">下一页</button>
      </div>
    </div>

    <!-- 添加/编辑密钥弹窗 -->
    <div v-if="showAddModal" class="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div class="card max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        <div class="p-6 border-b border-slate-700 flex justify-between items-center">
          <h3 class="text-xl font-bold">{{ editingKey ? '编辑公钥' : '添加公钥' }}</h3>
          <button class="text-slate-400 hover:text-white" @click="closeModal">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <form class="p-6 space-y-4" @submit.prevent="submitForm">
          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">密钥名称</label>
            <input v-model="form.name" type="text" class="input" placeholder="例: 我的SSH公钥" required />
          </div>

          <div>
            <label class="block text-sm font-medium text-slate-300 mb-2">公钥内容</label>
            <textarea
              v-model="form.publicKey"
              class="input min-h-[150px] font-mono text-sm"
              placeholder="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC..."
              required
            ></textarea>
            <p class="text-xs text-slate-400 mt-2">粘贴您的 SSH 公钥内容（通常位于 ~/.ssh/id_rsa.pub）</p>
          </div>

          <div class="flex gap-3 pt-4">
            <button type="button" class="btn btn-secondary flex-1" @click="closeModal">取消</button>
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

const keys = ref([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = 10
const totalPages = ref(0)
const searchText = ref('')
const filterType = ref('')
const showAddModal = ref(false)
const editingKey = ref(null)
const submitting = ref(false)

const form = ref({
  name: '',
  publicKey: ''
})

const loadKeys = async (page = 1) => {
  loading.value = true
  try {
    const response = await api.post('/key/list', {
      page,
      pageSize,
      name: searchText.value,
      keyType: filterType.value
    })
    keys.value = response.data.list || []
    currentPage.value = response.data.page
    totalPages.value = Math.ceil(response.data.total / pageSize)
  } catch (error) {
    toast.error(error.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = () => loadKeys(1)

const closeModal = () => {
  showAddModal.value = false
  editingKey.value = null
  form.value = { name: '', publicKey: '' }
}

const submitForm = async () => {
  submitting.value = true
  try {
    if (editingKey.value) {
      await api.post('/key/update', {
        id: editingKey.value.id,
        name: form.value.name,
        publicKey: form.value.publicKey
      })
      toast.success('更新成功')
    } else {
      await api.post('/key/create', {
        name: form.value.name,
        publicKey: form.value.publicKey
      })
      toast.success('添加成功')
    }
    closeModal()
    await loadKeys(currentPage.value)
  } catch (error) {
    toast.error(error.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const editKey = (key) => {
  editingKey.value = key
  form.value = {
    name: key.name,
    publicKey: key.publicKey
  }
  showAddModal.value = true
}

const deleteKey = async (id) => {
  if (!confirm('确定要删除此密钥吗？')) return
  try {
    await api.post('/key/delete', { ids: [id] })
    toast.success('删除成功')
    await loadKeys(currentPage.value)
  } catch (error) {
    toast.error(error.message || '删除失败')
  }
}

const copyToClipboard = (text) => {
  navigator.clipboard.writeText(text).then(() => {
    toast.success('已复制到剪贴板')
  }).catch(() => {
    toast.error('复制失败')
  })
}

onMounted(() => {
  loadKeys()
})
</script>
