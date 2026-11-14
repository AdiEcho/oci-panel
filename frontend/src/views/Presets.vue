<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useMotion } from '@vueuse/motion'
import { RouterLink } from 'vue-router'
import {
  Plus,
  Trash2,
  Edit,
  Loader2,
  Package,
  Cpu,
  HardDrive,
  MemoryStick,
  Key,
  MoreHorizontal,
  Settings2,
  RefreshCw,
  Check,
  Server
} from '@lucide/vue'
import { presetApi, keyApi, ociApi, type Preset, type ShapeInfo, type ConfigItem } from '@/api'
import { toast } from '@/composables/useToast'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'
import { Dropdown, DropdownItem } from '@/components/ui/dropdown'

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

// Oracle 规格卡片数据：默认仅展示免费规格，高级设置展示全部。
const ociConfigs = ref<ConfigItem[]>([])
const shapeSourceId = ref('')
const shapes = ref<ShapeInfo[]>([])
const loadingShapes = ref(false)
const showAdvanced = ref(false)

const blankForm = () => ({
  name: '',
  ocpus: 1,
  memory: 6,
  disk: 50,
  bootVolumeVpu: 10,
  architecture: 'ARM',
  shape: '',
  operationSystem: 'Ubuntu',
  imageId: '',
  sshKeyId: '',
  description: ''
})

const form = ref(blankForm())

const alwaysFreeShapeNames = ['VM.Standard.A1.Flex', 'VM.Standard.E2.1.Micro']

const fallbackShape = (architecture: string) =>
  architecture === 'AMD' ? 'VM.Standard.E2.1.Micro' : 'VM.Standard.A1.Flex'

const visibleShapes = computed(() => (showAdvanced.value ? shapes.value : shapes.value.filter(shape => shape.freeTier)))

const selectedShape = computed(() => shapes.value.find(shape => shape.shape === form.value.shape) || null)

const shapeMissing = computed(
  () => !!form.value.shape && !loadingShapes.value && shapes.value.length > 0 && !selectedShape.value
)

const isFlexSelected = computed(() => selectedShape.value?.isFlex ?? true)

const selectedLimits = computed(() => {
  const shape = selectedShape.value
  if (!shape) {
    return { minOcpus: 0.1, maxOcpus: 64, minMemory: 1, maxMemory: 1024 }
  }
  const useFreeCap = shape.freeTier && !showAdvanced.value
  return {
    minOcpus: shape.minOcpus || 0.1,
    maxOcpus: (useFreeCap ? shape.freeTierMaxOcpus : shape.maxOcpus) || 64,
    minMemory: shape.minMemoryInGBs || 1,
    maxMemory: (useFreeCap ? shape.freeTierMaxMemoryInGBs : shape.maxMemoryInGBs) || 1024
  }
})

const fmt = (value: number) => {
  if (!value) return '0'
  return Number.isInteger(value) ? String(value) : value.toFixed(1)
}

const shapeSummary = (shape: ShapeInfo) =>
  shape.isFlex
    ? `${fmt(shape.minOcpus)}-${fmt(shape.maxOcpus)} 核 · ${fmt(shape.minMemoryInGBs)}-${fmt(shape.maxMemoryInGBs)} GB`
    : `${fmt(shape.ocpus)} 核 · ${fmt(shape.memoryInGBs)} GB`

// 预估价格：官方按需单价（USD），前端按实际选择估算。
const money = (value: number) => value.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')

const hourlyFor = (shape: ShapeInfo, ocpus: number, memory: number): number | null => {
  if (!shape.price) return null
  if (shape.freeTier) {
    // 永久免费规格只对超出免费额度的部分计费。
    const extraOcpus = Math.max(0, ocpus - shape.freeTierMaxOcpus)
    const extraMemory = Math.max(0, memory - shape.freeTierMaxMemoryInGBs)
    if (extraOcpus === 0 && extraMemory === 0) return 0
    return extraOcpus * shape.price.ocpuHourly + extraMemory * shape.price.memoryGbHourly
  }
  if (shape.price.billedBy === 'gpu') {
    return shape.price.gpuHourly * Math.max(1, shape.gpus)
  }
  let hourly = ocpus * shape.price.ocpuHourly + memory * shape.price.memoryGbHourly
  if (shape.price.nvmeTbHourly > 0 && shape.localDisks > 0) {
    hourly += ((shape.localDisks * shape.localDiskSizeInGBs) / 1024) * shape.price.nvmeTbHourly
  }
  return hourly
}

const unitPrice = (shape: ShapeInfo) => {
  if (!shape.price) return ''
  if (shape.price.billedBy === 'gpu') {
    return `$${money(shape.price.gpuHourly)}/GPU·时`
  }
  if (shape.isFlex) {
    return `$${money(shape.price.ocpuHourly)}/核·时 + $${money(shape.price.memoryGbHourly)}/GB·时`
  }
  const hourly = hourlyFor(shape, shape.ocpus, shape.memoryInGBs)
  if (hourly === null) return ''
  if (hourly === 0) return '免费额度内'
  return `≈ $${(hourly * 730).toFixed(2)}/月`
}

const selectedHourly = computed(() => {
  const shape = selectedShape.value
  if (!shape) return null
  return hourlyFor(shape, form.value.ocpus, form.value.memory)
})

const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value))

const selectShape = (shape: ShapeInfo) => {
  form.value.shape = shape.shape
  form.value.architecture = shape.architecture || form.value.architecture
  const useFreeCap = shape.freeTier && !showAdvanced.value
  const maxOcpus = useFreeCap ? shape.freeTierMaxOcpus || shape.maxOcpus : shape.maxOcpus
  const maxMemory = useFreeCap ? shape.freeTierMaxMemoryInGBs || shape.maxMemoryInGBs : shape.maxMemoryInGBs

  if (shape.isFlex) {
    const defaultOcpus = useFreeCap ? shape.freeTierMaxOcpus || shape.ocpus : shape.ocpus
    const defaultMemory = useFreeCap ? shape.freeTierMaxMemoryInGBs || shape.memoryInGBs : shape.memoryInGBs
    form.value.ocpus = clamp(defaultOcpus || shape.minOcpus, shape.minOcpus || 0.1, maxOcpus || 64)
    form.value.memory = clamp(defaultMemory || shape.minMemoryInGBs, shape.minMemoryInGBs || 1, maxMemory || 1024)
  } else {
    form.value.ocpus = shape.ocpus || 1
    form.value.memory = shape.memoryInGBs || 1
  }
}

const loadConfigs = async () => {
  try {
    const response = await ociApi.userPage({ page: 1, pageSize: 100 })
    ociConfigs.value = response.data.list || []
    if (!shapeSourceId.value && ociConfigs.value.length > 0) {
      shapeSourceId.value = ociConfigs.value[0].id
    }
  } catch {
    ociConfigs.value = []
  }
}

const loadShapes = async (clearCache = false) => {
  const config = ociConfigs.value.find(item => item.id === shapeSourceId.value)
  if (!config) {
    shapes.value = []
    return
  }
  loadingShapes.value = true
  try {
    const response = await ociApi.shapes({ configId: config.id, region: config.ociRegion, clearCache })
    shapes.value = response.data || []
    if (!form.value.shape) {
      const firstFree = shapes.value.find(shape => shape.freeTier)
      if (firstFree) selectShape(firstFree)
    }
  } catch (error: any) {
    shapes.value = []
    toast.error(error.message || '获取规格列表失败')
  } finally {
    loadingShapes.value = false
  }
}

const onShapeSourceChange = () => {
  // 切换账号后规格集合会变，保留已选规格名称，由 shapeMissing 提示是否仍可用。
  loadShapes()
}

const toggleAdvanced = () => {
  showAdvanced.value = !showAdvanced.value
  if (!showAdvanced.value) {
    const freeShapes = shapes.value.filter(shape => shape.freeTier)
    if (!freeShapes.some(shape => shape.shape === form.value.shape)) {
      if (freeShapes.length > 0) {
        selectShape(freeShapes[0])
      } else {
        form.value.shape = fallbackShape(form.value.architecture)
      }
    }
  }
}

const loadPresets = async () => {
  loading.value = true
  try {
    const response = await presetApi.list()
    presets.value = response.data || []
  } catch (error: any) {
    toast.error(error.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const loadSSHKeys = async () => {
  try {
    const response = await keyApi.standalone()
    sshKeys.value = response.data || []
  } catch {
    sshKeys.value = []
  }
}

const openModal = async (preset?: Preset) => {
  editing.value = preset ?? null
  if (preset) {
    const shape = preset.shape || fallbackShape(preset.architecture)
    form.value = {
      name: preset.name,
      ocpus: preset.ocpus,
      memory: preset.memory,
      disk: preset.disk,
      bootVolumeVpu: preset.bootVolumeVpu,
      architecture: preset.architecture,
      shape,
      operationSystem: preset.operationSystem,
      imageId: preset.imageId,
      sshKeyId: preset.sshKeyId,
      description: preset.description
    }
    showAdvanced.value = !alwaysFreeShapeNames.includes(shape)
  } else {
    form.value = blankForm()
    showAdvanced.value = false
  }
  showModal.value = true

  if (ociConfigs.value.length === 0) {
    await loadConfigs()
  }
  await loadShapes()

  // 旧预设没有 shape 时按架构补默认；已选规格保持原值，不覆盖 CPU/内存。
  if (!form.value.shape) {
    form.value.shape = fallbackShape(form.value.architecture)
  }
  // 已存配置超出免费额度时自动展开高级设置，避免编辑时被免费上限误导。
  const shape = selectedShape.value
  if (!showAdvanced.value && shape) {
    const maxOcpus = shape.freeTierMaxOcpus || shape.maxOcpus || 64
    const maxMemory = shape.freeTierMaxMemoryInGBs || shape.maxMemoryInGBs || 1024
    if (form.value.ocpus > maxOcpus || form.value.memory > maxMemory) {
      showAdvanced.value = true
    }
  }
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
      await presetApi.update({ id: editing.value.id, ...form.value })
      toast.success('更新成功')
    } else {
      await presetApi.create(form.value)
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
    await presetApi.delete(id)
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
                <p v-if="preset.shape" class="text-[11px] font-mono text-muted-foreground mt-1 truncate max-w-[220px]">
                  {{ preset.shape }}
                </p>
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

    <Dialog v-model:open="showModal" size="xl">
      <DialogHeader class="mb-4">
        <DialogTitle>{{ editing ? '编辑预设' : '新建预设' }}</DialogTitle>
        <DialogDescription>选择 Oracle 可创建的实例规格，并配置实例创建时的默认参数</DialogDescription>
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="submitForm">
        <div>
          <label class="block text-sm font-medium mb-2">预设名称</label>
          <Input v-model="form.name" placeholder="例: ARM 4核24G" required />
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label class="block text-sm font-medium mb-2">规格查询账号</label>
            <select
              v-model="shapeSourceId"
              class="w-full h-10 px-3 rounded-md border border-input bg-background text-sm"
              @change="onShapeSourceChange"
            >
              <option value="" disabled>选择 OCI 配置</option>
              <option v-for="config in ociConfigs" :key="config.id" :value="config.id">
                {{ config.username }} ({{ config.ociRegion }})
              </option>
            </select>
          </div>
          <div class="flex items-end">
            <p class="text-xs text-muted-foreground">规格来自所选账号所在区域，结果会缓存；默认只显示永久免费规格。</p>
          </div>
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <label class="block text-sm font-medium">实例规格</label>
            <div class="flex items-center gap-2">
              <Button
                v-if="shapeSourceId"
                type="button"
                size="sm"
                variant="ghost"
                :disabled="loadingShapes"
                title="刷新规格缓存"
                @click="loadShapes(true)"
              >
                <RefreshCw :class="['w-3.5 h-3.5', loadingShapes && 'animate-spin']" />
              </Button>
              <Button type="button" size="sm" variant="outline" @click="toggleAdvanced">
                <Settings2 class="w-3.5 h-3.5" />
                {{ showAdvanced ? '仅看免费' : '高级设置' }}
              </Button>
            </div>
          </div>

          <div v-if="loadingShapes" class="flex items-center justify-center rounded-md border border-dashed h-32">
            <Loader2 class="w-6 h-6 animate-spin text-primary" />
          </div>

          <div
            v-else-if="visibleShapes.length"
            class="grid grid-cols-2 sm:grid-cols-3 gap-2 max-h-64 overflow-y-auto pr-1"
          >
            <button
              v-for="shape in visibleShapes"
              :key="shape.shape"
              type="button"
              :class="[
                'relative rounded-lg border p-3 text-left transition-colors',
                form.shape === shape.shape
                  ? 'border-primary bg-primary/5 ring-1 ring-primary'
                  : 'border-border hover:border-primary/50 hover:bg-accent/40'
              ]"
              @click="selectShape(shape)"
            >
              <div class="flex items-center justify-between gap-1">
                <div class="flex items-center gap-1">
                  <Badge :variant="shape.architecture === 'ARM' ? 'info' : 'secondary'">{{ shape.architecture }}</Badge>
                  <Badge v-if="shape.freeTier" variant="success">免费</Badge>
                </div>
                <Check v-if="form.shape === shape.shape" class="w-4 h-4 text-primary shrink-0" />
              </div>
              <p class="mt-2 text-sm font-medium truncate" :title="shape.shape">{{ shape.shape }}</p>
              <p class="text-xs text-muted-foreground">{{ shapeSummary(shape) }}</p>
              <p
                v-if="shape.price"
                class="text-[11px] font-medium text-primary"
                :title="`价格来源: ${shape.price.source} · ${shape.price.updatedAt}`"
              >
                {{ unitPrice(shape) }}
              </p>
              <p class="text-[11px] text-muted-foreground/80 truncate" :title="shape.processorDescription">
                {{ shape.processorDescription }}
              </p>
              <p v-if="shape.localDisks > 0" class="text-[11px] text-muted-foreground truncate">
                本地盘 {{ shape.localDisks }}× {{ fmt(shape.localDiskSizeInGBs) }}GB
              </p>
              <p v-if="shape.gpus > 0" class="text-[11px] text-warning truncate">
                {{ shape.gpus }}× {{ shape.gpuDescription }}
              </p>
            </button>
          </div>

          <div v-else class="rounded-md border border-dashed p-4 space-y-3">
            <div class="flex items-center gap-2 text-sm text-muted-foreground">
              <Server class="w-4 h-4" />
              <span v-if="!ociConfigs.length">
                暂无 OCI 配置，无法读取区域规格。
                <RouterLink to="/configs" class="text-primary hover:underline">先添加配置</RouterLink>
              </span>
              <span v-else-if="showAdvanced">该区域未返回可用规格，可手动填写 CPU 与内存。</span>
              <span v-else>该区域暂无免费规格，可开启高级设置查看全部规格。</span>
            </div>
            <div class="max-w-[200px]">
              <label class="block text-sm font-medium mb-2">架构</label>
              <select
                v-model="form.architecture"
                class="w-full h-10 px-3 rounded-md border border-input bg-background text-sm"
              >
                <option value="ARM">ARM</option>
                <option value="AMD">AMD</option>
              </select>
            </div>
          </div>

          <p v-if="shapeMissing" class="text-xs text-warning">
            已选规格 {{ form.shape }} 不在当前区域的可用列表中，提交后由后端按架构回退默认规格。
          </p>
          <p v-else-if="selectedShape?.freeTier" class="text-xs text-muted-foreground">
            免费额度：{{ selectedShape.freeTierMaxOcpus }} 核 / {{ selectedShape.freeTierMaxMemoryInGBs }} GB{{
              showAdvanced ? '（高级设置下可超出免费额度，按量计费）' : ''
            }}
          </p>
          <p v-if="selectedHourly !== null" class="text-xs text-muted-foreground">
            <span v-if="selectedHourly === 0">预估 $0.00/月（免费额度内）</span>
            <span v-else>
              预估 ≈ ${{ (selectedHourly * 730).toFixed(2) }}/月 · ${{ selectedHourly.toFixed(3) }}/小时
            </span>
            · 按需价 USD，730 小时/月，不含引导卷与网络
          </p>
        </div>

        <div>
          <label class="block text-sm font-medium mb-2">规格名称</label>
          <Input v-model="form.shape" placeholder="例: VM.Standard.A1.Flex" />
        </div>

        <div class="grid grid-cols-3 gap-4">
          <div>
            <label class="block text-sm font-medium mb-2">CPU核心数</label>
            <Input
              v-model.number="form.ocpus"
              type="number"
              step="1"
              :min="selectedLimits.minOcpus"
              :max="selectedLimits.maxOcpus"
              :disabled="!isFlexSelected"
              required
            />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">内存(GB)</label>
            <Input
              v-model.number="form.memory"
              type="number"
              step="1"
              :min="selectedLimits.minMemory"
              :max="selectedLimits.maxMemory"
              :disabled="!isFlexSelected"
              required
            />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">磁盘(GB)</label>
            <Input v-model.number="form.disk" type="number" min="50" required />
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4">
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

        <div v-if="showAdvanced" class="space-y-4 rounded-lg border border-dashed p-4">
          <p class="flex items-center gap-2 text-sm font-medium">
            <Settings2 class="w-4 h-4" />
            高级设置
          </p>
          <div class="grid grid-cols-2 gap-4">
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
              <label class="block text-sm font-medium mb-2">系统版本 (镜像ID)</label>
              <Input v-model="form.imageId" placeholder="留空则自动选择最新版本" />
            </div>
          </div>
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
