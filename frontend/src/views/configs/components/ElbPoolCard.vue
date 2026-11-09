<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Layers, Plus, RefreshCw, Loader2, Copy, Trash2, Server, AlertTriangle, Info, X } from '@lucide/vue'
import { ociApi } from '@/api'
import type { ElbPoolItem, VCNInfo } from '@/api/oci'
import { toast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { copyToClipboard } from '@/lib/utils'

// 配置详情「500Mbps ELB」卡片：先把 OCI 网络负载均衡器（NLB/ELB）建好，之后再在实例的
// 500Mbps 卡片里分配。创建时只占配额不绑定实例，分配后由实例开启流程接管端口映射。
const props = defineProps<{
  userId: string
}>()

const loading = ref(false)
const creating = ref(false)
const deletingId = ref('')
const showCreate = ref(false)
const pools = ref<ElbPoolItem[]>([])
const vcns = ref<VCNInfo[]>([])
const form = ref({ vcnId: '', nlbName: '' })
let pollTimer: ReturnType<typeof setTimeout> | undefined

const availableCount = computed(() => pools.value.filter(p => p.status === 'available').length)
const assignedCount = computed(() => pools.value.filter(p => p.status === 'assigned').length)

const statusMeta = (status: string) => {
  switch (status) {
    case 'provisioning':
      return { label: '创建中', variant: 'info' as const }
    case 'available':
      return { label: '待分配', variant: 'warning' as const }
    case 'assigned':
      return { label: '已分配', variant: 'success' as const }
    case 'missing':
      return { label: '云端丢失', variant: 'destructive' as const }
    case 'failed':
      return { label: '创建失败', variant: 'destructive' as const }
    default:
      return { label: status || '未知', variant: 'outline' as const }
  }
}

const stopPolling = () => {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = undefined
  }
}

// 创建中的 ELB 由后端异步等待，这里轻量轮询直到拿到公网 IP 或失败
const schedulePooling = () => {
  stopPolling()
  if (!pools.value.some(p => p.status === 'provisioning')) return
  pollTimer = setTimeout(async () => {
    await load(true)
    schedulePooling()
  }, 8000)
}

const load = async (silent = false) => {
  if (!props.userId) return
  if (!silent) loading.value = true
  try {
    const res = await ociApi.elbList(props.userId)
    pools.value = res.data || []
    schedulePooling()
  } catch (error: any) {
    toast.error(error.message || '加载 ELB 列表失败')
    pools.value = []
  } finally {
    loading.value = false
  }
}

const loadVcns = async () => {
  if (!props.userId || vcns.value.length) return
  try {
    const res = await ociApi.detailsVCNs({ configId: props.userId })
    vcns.value = res.data || []
    if (!form.value.vcnId && vcns.value.length) form.value.vcnId = vcns.value[0].id
  } catch {
    vcns.value = []
  }
}

const openCreate = async () => {
  showCreate.value = true
  await loadVcns()
}

const resetForm = () => {
  form.value = { vcnId: vcns.value[0]?.id || '', nlbName: '' }
  showCreate.value = false
}

const createPool = async () => {
  if (!form.value.vcnId) {
    toast.warning('请选择 ELB 所属 VCN')
    return
  }
  creating.value = true
  try {
    await ociApi.elbCreate({
      configId: props.userId,
      vcnId: form.value.vcnId,
      nlbName: form.value.nlbName.trim() || undefined
    })
    toast.info('ELB 创建任务已提交，通常 1-3 分钟完成，完成后状态变为「待分配」')
    resetForm()
    await load(true)
  } catch (error: any) {
    toast.error(error.message || '创建 ELB 失败')
    await load(true)
  } finally {
    creating.value = false
  }
}

const deletePool = async (pool: ElbPoolItem) => {
  if (
    !confirm(
      pool.status === 'available'
        ? `确定删除 ELB「${pool.nlbName || pool.nlbId}」吗？删除后需重新创建才能使用。`
        : `ELB「${pool.nlbName || pool.nlbId}」当前状态为 ${statusMeta(pool.status).label}，确定清理该记录/资源吗？`
    )
  )
    return
  deletingId.value = pool.nlbId
  try {
    await ociApi.elbDelete({ configId: props.userId, nlbId: pool.nlbId })
    toast.success('ELB 已删除')
    await load(true)
  } catch (error: any) {
    toast.error(error.message || '删除失败')
    await load(true)
  } finally {
    deletingId.value = ''
  }
}

const copyIp = async (ip?: string) => {
  if (!ip) return
  try {
    await copyToClipboard(ip)
    toast.success('访问 IP 已复制')
  } catch {
    toast.error('复制失败')
  }
}

onMounted(() => load())
onUnmounted(stopPolling)
</script>

<template>
  <Card class="p-4 bg-muted/30">
    <div class="flex items-center justify-between mb-3 flex-wrap gap-2">
      <div>
        <h4 class="text-sm font-semibold flex items-center gap-2">
          <Layers class="w-4 h-4 text-primary" />
          500Mbps ELB (网络负载均衡器)
        </h4>
        <p class="text-[11px] text-muted-foreground mt-1">
          先创建 ELB 占住配额，之后可分配给任意 AMD 实例；待分配 {{ availableCount }} · 已分配
          {{ assignedCount }}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <Button size="sm" variant="ghost" :disabled="loading" @click="load(true)">
          <RefreshCw :class="['w-4 h-4', loading && 'animate-spin']" />
        </Button>
        <Button size="sm" :disabled="!userId" @click="showCreate ? resetForm() : openCreate()">
          <X v-if="showCreate" class="w-4 h-4" />
          <Plus v-else class="w-4 h-4" />
          {{ showCreate ? '取消' : '新建ELB' }}
        </Button>
      </div>
    </div>

    <!-- 创建表单 -->
    <div v-if="showCreate" class="mb-3 rounded-lg border border-border bg-background/60 p-3 space-y-3">
      <div class="bg-info/10 border border-info/30 rounded-lg p-2.5 text-[11px] text-info flex items-start gap-1.5">
        <Info class="w-3.5 h-3.5 shrink-0 mt-0.5" />
        <span>
          只创建负载均衡器本身，不绑定机器、不占用 NAT/路由。创建后状态为「待分配」，可在实例的 500Mbps
          卡片里选择它开启，届时才接入端口映射与路由。
        </span>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div>
          <label class="block text-xs text-muted-foreground mb-1">所属 VCN</label>
          <Select v-model="form.vcnId" placeholder="选择 VCN">
            <option v-for="vcn in vcns" :key="vcn.id" :value="vcn.id">{{ vcn.displayName }}</option>
          </Select>
          <span class="text-[10px] text-muted-foreground mt-0.5 block">将自动选择该 VCN 下的公有子网</span>
        </div>
        <div>
          <label class="block text-xs text-muted-foreground mb-1">ELB 名称 (可选)</label>
          <Input v-model="form.nlbName" placeholder="oci-panel-shared-nlb-时间戳" />
        </div>
      </div>
      <Button class="w-full" :disabled="creating" @click="createPool">
        <Loader2 v-if="creating" class="w-4 h-4 animate-spin" />
        {{ creating ? '正在创建 ELB，约需 1-3 分钟...' : '创建 ELB' }}
      </Button>
    </div>

    <!-- 加载中 -->
    <div v-if="loading" class="flex items-center justify-center py-6 text-sm text-muted-foreground gap-2">
      <Loader2 class="w-5 h-5 animate-spin text-primary" />
      正在核验 ELB 与云端状态...
    </div>

    <!-- 空状态 -->
    <div
      v-else-if="!pools.length"
      class="rounded-lg bg-muted/50 p-3 text-xs text-muted-foreground flex items-center gap-2"
    >
      <Layers class="w-4 h-4 shrink-0" />
      <span>暂无 ELB。可先创建一个，再在实例的 500Mbps 卡片中分配使用。</span>
    </div>

    <!-- 列表 -->
    <div v-else class="space-y-2">
      <div
        v-for="pool in pools"
        :key="pool.nlbId"
        class="rounded-lg border border-border bg-background/60 p-3 text-xs space-y-2"
      >
        <div class="flex items-start justify-between gap-2 flex-wrap">
          <div class="min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="font-semibold text-sm truncate">{{ pool.nlbName || '未命名 ELB' }}</span>
              <Badge :variant="statusMeta(pool.status).variant" class="text-[10px] py-0 px-2">
                {{ statusMeta(pool.status).label }}
              </Badge>
              <Badge v-if="pool.origin === 'adopted'" variant="outline" class="text-[10px] py-0 px-2">纳管</Badge>
              <Badge v-if="pool.mode === 'dedicated'" variant="warning" class="text-[10px] py-0 px-2">独享全端口</Badge>
            </div>
            <div class="text-muted-foreground mt-1 flex items-center gap-3 flex-wrap">
              <span>{{ pool.vcnName || pool.vcnId || '未知 VCN' }}</span>
              <span v-if="pool.createdAt">创建于 {{ pool.createdAt }}</span>
              <span v-if="pool.memberCount">成员 {{ pool.memberCount }} 台</span>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <Button
              size="sm"
              variant="ghost"
              class="h-7 px-2 text-destructive hover:text-destructive"
              :disabled="deletingId === pool.nlbId || !pool.canDelete"
              :title="
                pool.status === 'missing'
                  ? '清理失效记录'
                  : pool.canDelete
                    ? '删除该 ELB'
                    : '仍有实例占用，需先关闭 500Mbps'
              "
              @click="deletePool(pool)"
            >
              <Loader2 v-if="deletingId === pool.nlbId" class="w-3.5 h-3.5 animate-spin" />
              <Trash2 v-else class="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>

        <div
          v-if="pool.accessIp && pool.status !== 'missing'"
          class="bg-muted/40 rounded p-2 flex items-center gap-2 font-mono text-[11px]"
        >
          <span class="text-muted-foreground font-sans">公网访问 IP:</span>
          <span class="text-primary break-all">{{ pool.accessIp }}</span>
          <Button variant="ghost" size="icon" class="h-6 w-6 shrink-0" @click="copyIp(pool.accessIp)">
            <Copy class="w-3 h-3" />
          </Button>
        </div>

        <div v-if="pool.instanceId" class="flex items-center gap-1.5 text-success">
          <Server class="w-3.5 h-3.5 shrink-0" />
          <span>
            已分配给 {{ pool.instanceName || pool.instanceId }}
            <template v-if="(pool.memberCount || 0) > 1">，共 {{ pool.memberCount }} 台实例共享</template>
          </span>
        </div>
        <div
          v-else-if="pool.status === 'available' && pool.canAssign"
          class="flex items-center gap-1.5 text-muted-foreground"
        >
          <Info class="w-3.5 h-3.5 shrink-0" />
          <span>待分配：在实例「编辑配置 → 500Mbps 下行带宽」中选择此 ELB 开启。</span>
        </div>

        <div
          v-if="pool.lastError"
          class="bg-destructive/10 border border-destructive/30 rounded p-2 text-destructive flex items-start gap-1.5"
        >
          <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
          <span class="break-all">{{ pool.lastError }}</span>
        </div>
      </div>
    </div>
  </Card>
</template>
