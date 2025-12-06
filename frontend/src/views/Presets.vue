<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useMotion } from '@vueuse/motion'
import { RouterLink } from 'vue-router'
import { Plus, Trash2, Edit, Loader2, Package, Cpu, HardDrive, MemoryStick, Key, MoreHorizontal } from 'lucide-vue-next'
import api from '@/lib/api'
import { toast } from '@/composables/useToast'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'
import { Dropdown, DropdownItem } from '@/components/ui/dropdown'

interface Preset {
  id: string
  name: string
  ocpus: number
  memory: number
  disk: number
  bootVolumeVpu: number
  architecture: string
  operationSystem: string
  imageId: string
  sshKeyId: string
  sshKeyName: string
  description: string
  createTime: string
}

interface SSHKey {
  id: string
  name: string
}

const presets = ref<Preset[]>([])
const sshKeys = ref<SSHKey[]>([])
const loading = ref(false)
const showModal = ref(false)
const editing = ref<Preset | null>(null)
const submitting = ref(false)

const form = ref({
  name: '',
  ocpus: 1,
  memory: 6,
  disk: 50,
  bootVolumeVpu: 10,
  architecture: 'ARM',
  operationSystem: 'Ubuntu',
  imageId: '',
  sshKeyId: '',
  description: ''
})

const loadPresets = async () => {
  loading.value = true
  try {
    const response = await api.get('/preset/list')
    presets.value = response.data || []
  } catch (error: any) {
    toast.error(error.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const loadSSHKeys = async () => {
  try {
    const response = await api.get('/key/standalone')
    sshKeys.value = response.data || []
  } catch {
    sshKeys.value = []
  }
}

const openModal = (preset?: Preset) => {
  if (preset) {
    editing.value = preset
    form.value = {
      name: preset.name,
      ocpus: preset.ocpus,
      memory: preset.memory,
      disk: preset.disk,
      bootVolumeVpu: preset.bootVolumeVpu,
      architecture: preset.architecture,
      operationSystem: preset.operationSystem,
      imageId: preset.imageId,
      sshKeyId: preset.sshKeyId,
      description: preset.description
    }
  } else {
    editing.value = null
    form.value = {
      name: '',
      ocpus: 1,
      memory: 6,
      disk: 50,
      bootVolumeVpu: 10,
      architecture: 'ARM',
      operationSystem: 'Ubuntu',
      imageId: '',
      sshKeyId: '',
      description: ''
    }
  }
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  editing.value = null
}

const submitForm = async () => {
  if (!form.value.name.trim()) {
    toast.warning('请输入预设名称')
    return
  }
  submitting.value = true
  try {
    if (editing.value) {
      await api.post('/preset/update', { id: editing.value.id, ...form.value })
      toast.success('更新成功')
    } else {
      await api.post('/preset/create', form.value)
      toast.success('创建成功')
    }
    closeModal()
    await loadPresets()
  } catch (error: any) {
    toast.error(error.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const deletePreset = async (id: string) => {
  if (!confirm('确定要删除此预设吗？')) return
  try {
    await api.post('/preset/delete', { id })
    toast.success('删除成功')
    await loadPresets()
  } catch (error: any) {
    toast.error(error.message || '删除失败')
  }
}

onMounted(() => {
  loadPresets()
  loadSSHKeys()
})

const headerRef = ref<HTMLElement>()
useMotion(headerRef, { initial: { opacity: 0, y: -20 }, enter: { opacity: 1, y: 0 } })
</script>

<template>
  <div class="space-y-6">
    <div ref="headerRef" class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div class="flex items-center gap-4">
        <h1 class="text-3xl font-display font-bold">预设配置</h1>
        <Badge variant="secondary">{{ presets.length }} 个预设</Badge>
      </div>
      <Button @click="openModal()">
        <Plus class="w-4 h-4" />
        新建预设
      </Button>
    </div>

    <Card
      v-motion
      :initial="{ opacity: 0, y: 20 }"
      :enter="{ opacity: 1, y: 0, transition: { delay: 100 } }"
      class="border-border/50"
    >
      <CardContent class="p-0">
        <Table>
          <TableHeader>
            <TableRow class="hover:bg-transparent">
              <TableHead>预设名称</TableHead>
              <TableHead>配置</TableHead>
              <TableHead>架构/系统</TableHead>
              <TableHead>SSH密钥</TableHead>
              <TableHead class="hidden lg:table-cell">创建时间</TableHead>
              <TableHead class="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-if="loading">
              <TableCell colspan="6" class="h-32 text-center">
                <Loader2 class="w-8 h-8 mx-auto animate-spin text-primary" />
              </TableCell>
            </TableRow>
            <TableRow v-else-if="!presets.length">
              <TableCell colspan="6" class="h-32 text-center text-muted-foreground">
                <div class="flex flex-col items-center gap-2">
                  <Package class="w-12 h-12 text-muted-foreground/50" />
                  <p>暂无预设配置</p>
                  <Button variant="outline" size="sm" @click="openModal()">
                    <Plus class="w-4 h-4" />
                    创建第一个预设
                  </Button>
                </div>
              </TableCell>
            </TableRow>
            <TableRow
              v-for="(preset, index) in presets"
              v-else
              :key="preset.id"
              v-motion
              :initial="{ opacity: 0, x: -20 }"
              :enter="{ opacity: 1, x: 0, transition: { delay: 50 * index } }"
            >
              <TableCell>
                <div>
                  <p class="font-medium">{{ preset.name }}</p>
                  <p v-if="preset.description" class="text-xs text-muted-foreground truncate max-w-[200px]">
                    {{ preset.description }}
                  </p>
                </div>
              </TableCell>
              <TableCell>
                <div class="flex items-center gap-3 text-sm">
                  <span class="flex items-center gap-1">
                    <Cpu class="w-3.5 h-3.5 text-primary" />
                    {{ preset.ocpus }}核
                  </span>
                  <span class="flex items-center gap-1">
                    <MemoryStick class="w-3.5 h-3.5 text-success" />
                    {{ preset.memory }}GB
                  </span>
                  <span class="flex items-center gap-1">
                    <HardDrive class="w-3.5 h-3.5 text-warning" />
                    {{ preset.disk }}GB
                  </span>
                </div>
              </TableCell>
              <TableCell>
                <div class="flex gap-1">
                  <Badge variant="info">{{ preset.architecture }}</Badge>
                  <Badge variant="secondary">{{ preset.operationSystem }}</Badge>
                </div>
              </TableCell>
              <TableCell>
                <span v-if="preset.sshKeyName" class="flex items-center gap-1 text-sm">
                  <Key class="w-3.5 h-3.5" />
                  {{ preset.sshKeyName }}
                </span>
                <span v-else class="text-muted-foreground text-sm">未设置</span>
              </TableCell>
              <TableCell class="text-sm text-muted-foreground hidden lg:table-cell">
                {{ preset.createTime }}
              </TableCell>
              <TableCell class="text-right">
                <div class="flex justify-end items-center gap-1">
                  <Button size="sm" variant="outline" @click="openModal(preset)">
                    <Edit class="w-3.5 h-3.5" />
                    <span class="hidden sm:inline ml-1">编辑</span>
                  </Button>
                  <Dropdown align="right">
                    <template #trigger>
                      <Button size="sm" variant="ghost"><MoreHorizontal class="w-4 h-4" /></Button>
                    </template>
                    <DropdownItem destructive @click="deletePreset(preset.id)">
                      <Trash2 class="w-4 h-4" />
                      删除预设
                    </DropdownItem>
                  </Dropdown>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>

    <Dialog v-model:open="showModal">
      <DialogHeader class="mb-4">
        <DialogTitle>{{ editing ? '编辑预设' : '新建预设' }}</DialogTitle>
        <DialogDescription>配置实例创建时的默认参数</DialogDescription>
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="submitForm">
        <div>
          <label class="block text-sm font-medium mb-2">预设名称</label>
          <Input v-model="form.name" placeholder="例: ARM 4核24G" required />
        </div>
        <div class="grid grid-cols-3 gap-4">
          <div>
            <label class="block text-sm font-medium mb-2">CPU核心数</label>
            <Input v-model.number="form.ocpus" type="number" step="0.1" min="0.1" required />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">内存(GB)</label>
            <Input v-model.number="form.memory" type="number" step="0.1" min="0.1" required />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">磁盘(GB)</label>
            <Input v-model.number="form.disk" type="number" min="50" required />
          </div>
        </div>
        <div class="grid grid-cols-3 gap-4">
          <div>
            <label class="block text-sm font-medium mb-2">VPU/GB</label>
            <select
              v-model.number="form.bootVolumeVpu"
              class="w-full h-10 px-3 rounded-md border border-input bg-background text-sm"
            >
              <option v-for="vpu in [10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120]" :key="vpu" :value="vpu">
                {{ vpu }}
              </option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">架构</label>
            <select
              v-model="form.architecture"
              class="w-full h-10 px-3 rounded-md border border-input bg-background text-sm"
            >
              <option value="ARM">ARM</option>
              <option value="AMD">AMD</option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">操作系统</label>
            <select
              v-model="form.operationSystem"
              class="w-full h-10 px-3 rounded-md border border-input bg-background text-sm"
            >
              <option value="Ubuntu">Ubuntu</option>
              <option value="CentOS">CentOS</option>
              <option value="Oracle Linux">Oracle Linux</option>
            </select>
          </div>
        </div>
        <div>
          <label class="block text-sm font-medium mb-2">系统版本 (镜像ID)</label>
          <Input v-model="form.imageId" placeholder="留空则自动选择最新版本" />
          <p class="text-xs text-muted-foreground mt-1">可在创建实例时从配置详情获取镜像ID</p>
        </div>
        <div>
          <label class="block text-sm font-medium mb-2">SSH公钥</label>
          <select v-model="form.sshKeyId" class="w-full h-10 px-3 rounded-md border border-input bg-background text-sm">
            <option value="">不预设</option>
            <option v-for="key in sshKeys" :key="key.id" :value="key.id">{{ key.name }}</option>
          </select>
          <p class="text-xs text-muted-foreground mt-1">
            <RouterLink to="/keys" class="text-primary hover:underline">管理密钥</RouterLink>
          </p>
        </div>
        <div>
          <label class="block text-sm font-medium mb-2">备注说明</label>
          <Textarea v-model="form.description" placeholder="可选的备注信息" :rows="2" />
        </div>
        <DialogFooter class="mt-6">
          <Button type="button" variant="outline" @click="closeModal">取消</Button>
          <Button type="submit" :disabled="submitting">
            <Loader2 v-if="submitting" class="w-4 h-4 animate-spin" />
            {{ submitting ? '提交中...' : editing ? '更新' : '创建' }}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  </div>
</template>
