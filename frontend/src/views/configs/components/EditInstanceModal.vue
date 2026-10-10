<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { Loader2, AlertTriangle, Settings, HardDrive, Globe, Wrench, Zap, Info, ShieldCheck } from '@lucide/vue'
import { instanceApi } from '@/api'
import { toast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Modal } from '@/components/ui/modal'
import { Badge } from '@/components/ui/badge'
import type { Instance } from '@/views/configs/types'
import type { Bandwidth500Summary } from '@/api/oci'

const props = defineProps<{
  open: boolean
  instance: Instance | null
  userId: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  refresh: []
}>()

const updating = ref(false)
const bwLoading = ref(true)
const bwStatus = ref<Bandwidth500Summary | null>(null)
let bwRequestId = 0

const form = reactive({
  displayName: '',
  ocpus: 2,
  memoryInGBs: 12,
  bootVolumeSize: 50,
  vpusPerGB: 10,
  currentIpv6: '',
  keepBackup: false
})

const bwForm = reactive({
  mode: 'shared-ports', // 默认推荐共享模式以规避配额限制
  listenPort: 2202,
  backendPort: 22,
  sshPort: 22,
  retainNatGw: true, // 默认安全保留
  retainNlb: true
})

const fetchBwStatus = async (userId: string, instanceId: string) => {
  const currentReq = ++bwRequestId
  bwLoading.value = true
  try {
    const res = await instanceApi.getStatus500Mbps(userId, instanceId)
    if (currentReq === bwRequestId && res.data) {
      bwStatus.value = res.data
      if (res.data.endpoints && res.data.endpoints.length > 0) {
        const ep = res.data.endpoints[0]
        bwForm.listenPort = ep.listenPort
        bwForm.backendPort = ep.backendPort
      }
    }
  } catch (error: any) {
    if (currentReq === bwRequestId) {
      bwStatus.value = {
        status: 'unknown',
        enabled: null,
        canEnable: false,
        canDisable: false,
        actionBlockedReason: error.message || '获取状态失败'
      }
    }
  } finally {
    if (currentReq === bwRequestId) {
      bwLoading.value = false
    }
  }
}

watch(
  () => [props.open, props.userId, props.instance?.id],
  ([open, userId, instanceId]) => {
    if (open && props.instance) {
      form.displayName = props.instance.displayName
      form.ocpus = props.instance.ocpus || 2
      form.memoryInGBs = props.instance.memory || 12
      form.bootVolumeSize = props.instance.bootVolumeSize || 50
      form.vpusPerGB = props.instance.bootVolumeVpu || 10
      form.currentIpv6 = props.instance.ipv6 || ''

      if (userId && instanceId) {
        fetchBwStatus(userId as string, instanceId as string)
      }
    } else {
      bwStatus.value = null
      bwLoading.value = false
    }
  },
  { immediate: true }
)

const close = () => emit('update:open', false)

const updateInstanceName = async () => {
  if (!form.displayName) {
    toast.warning('请输入实例名称')
    return
  }
  updating.value = true
  try {
    await instanceApi.updateName({
      userId: props.userId,
      instanceId: props.instance?.id,
      displayName: form.displayName
    })
    toast.success('实例名称更新成功')
    emit('refresh')
  } catch (error: any) {
    toast.error(error.message || '更新失败')
  } finally {
    updating.value = false
  }
}

const updateInstanceConfig = async () => {
  updating.value = true
  try {
    await instanceApi.updateConfig({
      userId: props.userId,
      instanceId: props.instance?.id,
      ocpus: form.ocpus,
      memoryInGBs: form.memoryInGBs
    })
    toast.success('实例配置更新成功')
    emit('refresh')
  } catch (error: any) {
    toast.error(error.message || '更新失败')
  } finally {
    updating.value = false
  }
}

const updateBootVolume = async () => {
  updating.value = true
  try {
    await instanceApi.updateBootVolume({
      userId: props.userId,
      instanceId: props.instance?.id,
      sizeInGBs: form.bootVolumeSize,
      vpusPerGB: form.vpusPerGB
    })
    toast.success('引导卷配置更新成功')
    emit('refresh')
  } catch (error: any) {
    toast.error(error.message || '更新失败')
  } finally {
    updating.value = false
  }
}

const attachIPv6 = async () => {
  if (form.currentIpv6) {
    toast.warning('该实例已有IPv6地址')
    return
  }
  if (!confirm('确定要为该实例附加IPv6地址吗？')) return
  updating.value = true
  try {
    const response = await instanceApi.attachIPv6({
      userId: props.userId,
      instanceId: props.instance?.id
    })
    if (response.data?.ipv6) {
      form.currentIpv6 = response.data.ipv6
      toast.success(`IPv6附加成功: ${response.data.ipv6}`)
    } else {
      toast.success('IPv6附加成功')
    }
    emit('refresh')
  } catch (error: any) {
    toast.error(error.message || '附加失败')
  } finally {
    updating.value = false
  }
}

const startAutoRescue = async () => {
  if (!confirm('自动救援将会关闭实例，备份引导卷，创建47GB新引导卷并重新启动。此操作需要5-10分钟，确定继续吗？')) return
  updating.value = true
  try {
    await instanceApi.autoRescue({
      userId: props.userId,
      instanceId: props.instance?.id,
      instanceName: form.displayName,
      keepBackup: form.keepBackup
    })
    toast.success('自动救援任务已启动，请等待5-10分钟完成')
    close()
  } catch (error: any) {
    toast.error(error.message || '启动失败')
  } finally {
    updating.value = false
  }
}

const enable500Mbps = async () => {
  if (!props.instance?.id) return
  if (
    !confirm(
      bwForm.mode === 'shared-ports'
        ? `将通过同 VCN 共享 NLB（监听端口 ${bwForm.listenPort} -> 后端 ${bwForm.backendPort}）及 NAT 网关开启 500Mbps，复用现有 NLB 避免配额超限，确定继续吗？`
        : '此操作将创建独享 NAT 网关和网络负载均衡器（占用 1 个 NLB 配额）来实现 500Mbps 下行带宽。确定继续吗？'
    )
  )
    return

  updating.value = true
  try {
    const mappings =
      bwForm.mode === 'shared-ports'
        ? [
            {
              id: 'ssh',
              protocol: 'TCP',
              listenPort: bwForm.listenPort,
              backendPort: bwForm.backendPort || 22
            }
          ]
        : []

    await instanceApi.enable500Mbps({
      userId: props.userId,
      instanceId: props.instance.id,
      sshPort: bwForm.sshPort || 22,
      mode: bwForm.mode,
      mappings
    })
    toast.success('500Mbps 开启任务已启动，请稍候在列表中查看 NLB 访问入口')
    close()
    emit('refresh')
  } catch (error: any) {
    toast.error(error.message || '开启失败')
  } finally {
    updating.value = false
  }
}

const disable500Mbps = async () => {
  if (!props.instance?.id) return
  if (!confirm('确定要为该实例关闭 500Mbps 吗？实例将恢复原始路由。若保留 NLB/NAT，其他实例的访问不受影响。')) return

  updating.value = true
  try {
    await instanceApi.disable500Mbps({
      userId: props.userId,
      instanceId: props.instance.id,
      retainNatGw: bwForm.retainNatGw,
      retainNlb: bwForm.retainNlb
    })
    toast.success('500Mbps 关闭任务已启动，正在恢复实例网络')
    close()
    emit('refresh')
  } catch (error: any) {
    toast.error(error.message || '关闭失败')
  } finally {
    updating.value = false
  }
}

const migrateToShared = async () => {
  if (!props.instance?.id) return
  if (
    !confirm(
      '将把该实例现有的独享 NLB 原地改造为受管共享池（保留公网 IP 不变），并将通配端口转换为显式端口映射（默认 SSH 22）。完成后其他实例可直接共享该 NLB，确定继续吗？'
    )
  )
    return

  updating.value = true
  try {
    await instanceApi.migrate500Mbps({
      userId: props.userId,
      instanceId: props.instance.id,
      sshPort: bwForm.sshPort || 22,
      mappings: [
        {
          id: 'ssh',
          protocol: 'TCP',
          listenPort: 22,
          backendPort: bwForm.sshPort || 22
        }
      ]
    })
    toast.success('已启动原地迁移任务，请稍候刷新')
    close()
    emit('refresh')
  } catch (error: any) {
    toast.error(error.message || '迁移失败')
  } finally {
    updating.value = false
  }
}
</script>

<template>
  <Modal
    :open="open"
    title="编辑实例配置"
    max-width="max-w-2xl"
    panel-class="max-h-[90vh] flex flex-col"
    @update:open="emit('update:open', $event)"
  >
    <div class="flex-1 overflow-y-auto p-6 space-y-4">
      <!-- 实例名称 -->
      <Card class="p-4 bg-muted/30">
        <h4 class="text-sm font-semibold mb-3 flex items-center gap-2">
          <Settings class="w-4 h-4 text-primary" />
          修改实例名称
        </h4>
        <div class="flex gap-3">
          <Input v-model="form.displayName" placeholder="请输入实例名称" class="flex-1" />
          <Button :disabled="updating" @click="updateInstanceName">
            <Loader2 v-if="updating" class="w-4 h-4 animate-spin" />
            修改名称
          </Button>
        </div>
      </Card>

      <!-- CPU和内存 -->
      <Card class="p-4 bg-muted/30">
        <h4 class="text-sm font-semibold mb-3 flex items-center gap-2">
          <Settings class="w-4 h-4 text-primary" />
          修改CPU和内存
        </h4>
        <div
          class="bg-warning/10 border border-warning/30 rounded-lg p-3 mb-3 text-xs text-warning flex items-center gap-2"
        >
          <AlertTriangle class="w-4 h-4 shrink-0" />
          修改CPU和内存需要停止实例
        </div>
        <div class="grid grid-cols-2 gap-3 mb-3">
          <div>
            <label class="block text-xs text-muted-foreground mb-1">CPU核心数 (OCPUs)</label>
            <Input v-model.number="form.ocpus" type="number" min="1" max="64" />
          </div>
          <div>
            <label class="block text-xs text-muted-foreground mb-1">内存 (GB)</label>
            <Input v-model.number="form.memoryInGBs" type="number" min="1" max="1024" />
          </div>
        </div>
        <Button class="w-full" :disabled="updating" @click="updateInstanceConfig">
          <Loader2 v-if="updating" class="w-4 h-4 animate-spin" />
          修改CPU和内存
        </Button>
      </Card>

      <!-- 引导卷配置 -->
      <Card class="p-4 bg-muted/30">
        <h4 class="text-sm font-semibold mb-3 flex items-center gap-2">
          <HardDrive class="w-4 h-4 text-primary" />
          修改引导卷大小及VPU
        </h4>
        <div class="grid grid-cols-2 gap-3 mb-3">
          <div>
            <label class="block text-xs text-muted-foreground mb-1">引导卷大小 (GB)</label>
            <Input v-model.number="form.bootVolumeSize" type="number" min="50" max="32768" />
          </div>
          <div>
            <label class="block text-xs text-muted-foreground mb-1">VPU/GB (性能)</label>
            <select
              v-model.number="form.vpusPerGB"
              class="w-full h-10 px-3 rounded-md border border-input bg-background text-sm"
            >
              <option v-for="v in [10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120]" :key="v" :value="v">
                {{ v }} VPU
              </option>
            </select>
          </div>
        </div>
        <Button class="w-full" :disabled="updating" @click="updateBootVolume">
          <Loader2 v-if="updating" class="w-4 h-4 animate-spin" />
          修改引导卷配置
        </Button>
      </Card>

      <!-- IPv6 -->
      <Card class="p-4 bg-muted/30">
        <h4 class="text-sm font-semibold mb-3 flex items-center gap-2">
          <Globe class="w-4 h-4 text-primary" />
          附加IPv6地址
        </h4>
        <div class="flex items-center justify-between">
          <div class="text-sm text-muted-foreground">
            <span v-if="form.currentIpv6">
              当前IPv6:
              <span class="text-primary font-mono">{{ form.currentIpv6 }}</span>
            </span>
            <span v-else>该实例未分配IPv6地址</span>
          </div>
          <Button :disabled="updating || !!form.currentIpv6" @click="attachIPv6">
            <Loader2 v-if="updating" class="w-4 h-4 animate-spin" />
            {{ form.currentIpv6 ? '已有IPv6' : '附加IPv6' }}
          </Button>
        </div>
      </Card>

      <!-- 自动救援 -->
      <Card class="p-4 bg-muted/30">
        <h4 class="text-sm font-semibold mb-3 flex items-center gap-2">
          <Wrench class="w-4 h-4 text-primary" />
          自动救援/缩小硬盘
        </h4>
        <div class="bg-warning/10 border border-warning/30 rounded-lg p-3 mb-3 text-xs text-warning">
          此操作会关闭实例，备份引导卷，创建47GB新引导卷并重新启动。过程需要5-10分钟。
        </div>
        <div class="flex items-center justify-between">
          <label class="flex items-center gap-2 text-sm cursor-pointer">
            <Checkbox v-model="form.keepBackup" />
            <span>保留原引导卷备份</span>
          </label>
          <Button variant="warning" :disabled="updating" @click="startAutoRescue">
            <Loader2 v-if="updating" class="w-4 h-4 animate-spin" />
            开始救援
          </Button>
        </div>
      </Card>

      <!-- 500Mbps 下行带宽 (严格互斥渲染) -->
      <Card class="p-4 bg-muted/30">
        <div class="flex items-center justify-between mb-3">
          <h4 class="text-sm font-semibold flex items-center gap-2">
            <Zap class="w-4 h-4 text-primary" />
            500Mbps下行带宽 (仅AMD实例)
          </h4>
          <Button
            v-if="!bwLoading && props.userId && props.instance?.id"
            size="sm"
            variant="ghost"
            class="h-7 text-xs px-2 text-muted-foreground"
            @click="fetchBwStatus(props.userId, props.instance.id)"
          >
            刷新状态
          </Button>
        </div>

        <!-- 1. 加载中状态：禁止闪现任何操作按钮 -->
        <div
          v-if="bwLoading"
          class="flex flex-col items-center justify-center py-6 text-muted-foreground text-sm space-y-2"
        >
          <Loader2 class="w-6 h-6 animate-spin text-primary" />
          <span>正在核验 500Mbps 状态与云端拓扑...</span>
        </div>

        <!-- 2. 机型不支持 -->
        <div
          v-else-if="props.instance?.shape && !props.instance.shape.includes('E2.1.Micro')"
          class="bg-muted/50 rounded-lg p-3 text-xs text-muted-foreground flex items-center gap-2"
        >
          <Info class="w-4 h-4 shrink-0 text-muted-foreground" />
          <span>此实例机型不支持 500Mbps 功能，仅 VM.Standard.E2.1.Micro 实例支持。</span>
        </div>

        <!-- 3. 进行中状态 -->
        <div
          v-else-if="
            bwStatus?.status === 'enabling' || bwStatus?.status === 'disabling' || bwStatus?.status === 'migrating'
          "
          class="bg-info/10 border border-info/30 rounded-lg p-3 space-y-2 text-xs text-info"
        >
          <div class="flex items-center gap-2 font-semibold">
            <Loader2 class="w-4 h-4 animate-spin shrink-0" />
            <span>当前正在执行操作: {{ bwStatus.status }}，请等待完成...</span>
          </div>
          <p class="text-muted-foreground">任务正在 OCI 云端异步执行中，请稍候在实例列表或刷新状态查看结果。</p>
        </div>

        <!-- 4. 异常或待修复状态 -->
        <div
          v-else-if="bwStatus?.status === 'repair-required' || bwStatus?.status === 'unknown'"
          class="bg-destructive/10 border border-destructive/30 rounded-lg p-3 space-y-2 text-xs text-destructive"
        >
          <div class="flex items-center gap-2 font-semibold">
            <AlertTriangle class="w-4 h-4 shrink-0" />
            <span>500Mbps 状态异常或待修复</span>
          </div>
          <p class="text-muted-foreground">
            {{ bwStatus.actionBlockedReason || '无法核验云端 NLB/路由配置，请检查凭据权限或云端资源' }}
          </p>
          <Button
            size="sm"
            variant="outline"
            class="h-7 text-xs mt-1"
            @click="fetchBwStatus(props.userId, props.instance!.id)"
          >
            重新核验
          </Button>
        </div>

        <!-- 5. 确认已启用状态：只显示“关闭500Mbps”按钮 -->
        <div v-else-if="bwStatus?.enabled === true" class="space-y-3">
          <div class="bg-success/10 border border-success/30 rounded-lg p-3 text-xs space-y-2">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-success flex items-center gap-1.5">
                <ShieldCheck class="w-4 h-4" />
                500Mbps 已启用 ({{ bwStatus.mode === 'shared-ports' ? '共享 NLB' : '独享 NLB' }})
              </span>
              <Badge variant="success" class="text-[11px] py-0 px-2">正常生效中</Badge>
            </div>
            <div class="text-foreground">
              <span class="text-muted-foreground">NLB 公网访问 IP:</span>
              <span class="font-mono font-bold text-primary">{{ bwStatus.accessIp || '获取中...' }}</span>
            </div>
            <div v-if="bwStatus.endpoints?.length" class="text-foreground">
              <span class="text-muted-foreground">映射端口:</span>
              <span
                v-for="ep in bwStatus.endpoints"
                :key="ep.id"
                class="font-mono bg-background/80 px-1.5 py-0.5 rounded border border-border mr-1.5"
              >
                {{ ep.protocol }} {{ ep.listenPort }} &rarr; {{ ep.backendPort }}
              </span>
            </div>
          </div>

          <!-- 独享转共享提示卡片 -->
          <div
            v-if="bwStatus.canMigrateToShared"
            class="bg-primary/5 border border-primary/20 rounded-lg p-2.5 text-xs space-y-1.5"
          >
            <div class="flex items-center justify-between">
              <span class="font-medium text-primary">可升级为共享池 (复用额度)</span>
              <Button
                size="sm"
                variant="outline"
                class="h-6 text-[11px] px-2 text-primary border-primary/30 hover:bg-primary/10"
                :disabled="updating"
                @click="migrateToShared"
              >
                升级为共享NLB
              </Button>
            </div>
            <p class="text-muted-foreground text-[11px]">
              将当前独享 NLB 原地升级为共享池（IP 不变），升级后第二台实例即可直接共享该 NLB，彻底避开配额超限。
            </p>
          </div>

          <!-- 关闭警告或阻止原因提示 -->
          <div
            v-if="!bwStatus.canDisable"
            class="bg-warning/10 border border-warning/30 rounded-lg p-2.5 text-xs text-warning flex items-start gap-2"
          >
            <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
            <span>
              {{
                bwStatus.actionBlockedReason ||
                '已识别历史受管 500Mbps 配置，但无初始路由快照。请先在控制台或迁移中确认恢复目标后再关闭。'
              }}
            </span>
          </div>

          <!-- 关闭操作与选项 -->
          <div class="pt-1">
            <Button
              variant="destructive"
              class="w-full"
              :disabled="updating || !bwStatus.canDisable"
              @click="disable500Mbps"
            >
              <Loader2 v-if="updating" class="w-4 h-4 animate-spin mr-1.5" />
              关闭500Mbps
            </Button>
            <div class="flex items-center gap-4 text-xs text-muted-foreground mt-2.5">
              <label class="flex items-center gap-1.5 cursor-pointer">
                <Checkbox v-model="bwForm.retainNatGw" />
                <span>保留NAT网关 (推荐)</span>
              </label>
              <label class="flex items-center gap-1.5 cursor-pointer">
                <Checkbox v-model="bwForm.retainNlb" />
                <span>保留负载均衡器 (推荐)</span>
              </label>
            </div>
          </div>
        </div>

        <!-- 6. 确认未启用状态：只显示“开启500Mbps”按钮 -->
        <div v-else class="space-y-3">
          <div class="bg-info/10 border border-info/30 rounded-lg p-3 text-xs text-info space-y-1">
            <div class="font-semibold flex items-center gap-1.5">
              <Info class="w-3.5 h-3.5" />
              通过 NAT 网关和网络负载均衡器实现 500Mbps 下行带宽
            </div>
            <p class="text-muted-foreground text-[11px]">
              推荐使用「共享端口」模式：多台 AMD 实例共用同一 NLB 公网 IP，通过不同外部端口访问各自的 SSH，极大避免触碰
              OCI 区域 NLB 数量限制。
            </p>
          </div>

          <!-- 模式选择 -->
          <div class="grid grid-cols-2 gap-2 text-xs">
            <label
              class="border rounded-lg p-2.5 cursor-pointer transition-colors flex flex-col gap-1"
              :class="
                bwForm.mode === 'shared-ports'
                  ? 'border-primary bg-primary/5 text-primary'
                  : 'border-border bg-background'
              "
            >
              <div class="flex items-center gap-2">
                <input v-model="bwForm.mode" type="radio" value="shared-ports" class="text-primary" />
                <span class="font-medium">共享端口 (推荐)</span>
              </div>
              <span class="text-[11px] text-muted-foreground">共用 NLB，独立端口转发，节省配额</span>
            </label>
            <label
              class="border rounded-lg p-2.5 cursor-pointer transition-colors flex flex-col gap-1"
              :class="
                bwForm.mode === 'dedicated' ? 'border-primary bg-primary/5 text-primary' : 'border-border bg-background'
              "
            >
              <div class="flex items-center gap-2">
                <input v-model="bwForm.mode" type="radio" value="dedicated" class="text-primary" />
                <span class="font-medium">独享全端口</span>
              </div>
              <span class="text-[11px] text-muted-foreground">全端口透明转发，需占用 1 个 NLB 配额</span>
            </label>
          </div>

          <!-- 共享模式端口配置 -->
          <div
            v-if="bwForm.mode === 'shared-ports'"
            class="grid grid-cols-2 gap-3 p-3 bg-background/50 rounded-lg border border-border/60"
          >
            <div>
              <label class="block text-xs text-muted-foreground mb-1">外部访问端口 (NLB)</label>
              <Input v-model.number="bwForm.listenPort" type="number" min="1" max="65535" placeholder="2202" />
              <span class="text-[10px] text-muted-foreground mt-0.5 block">外部客户端连接端口</span>
            </div>
            <div>
              <label class="block text-xs text-muted-foreground mb-1">实例内部端口 (SSH)</label>
              <Input v-model.number="bwForm.backendPort" type="number" min="1" max="65535" placeholder="22" />
              <span class="text-[10px] text-muted-foreground mt-0.5 block">转发至实例内的端口</span>
            </div>
          </div>

          <div v-else class="flex items-center gap-2">
            <label class="text-xs text-muted-foreground whitespace-nowrap">实例健康检查端口:</label>
            <Input
              v-model.number="bwForm.sshPort"
              type="number"
              min="1"
              max="65535"
              class="w-24 h-8 text-xs"
              placeholder="22"
            />
          </div>

          <Button variant="success" class="w-full" :disabled="updating" @click="enable500Mbps">
            <Loader2 v-if="updating" class="w-4 h-4 animate-spin mr-1.5" />
            开启500Mbps
          </Button>
        </div>
      </Card>
    </div>

    <div class="p-6 border-t border-border">
      <Button variant="outline" class="w-full" @click="close">关闭</Button>
    </div>
  </Modal>
</template>
