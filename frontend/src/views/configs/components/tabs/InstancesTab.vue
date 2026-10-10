<script setup lang="ts">
import { Loader2, Server, Play, Square, RotateCcw, Globe, Settings, Terminal, Trash2, Copy, Zap } from '@lucide/vue'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { copyToClipboard } from '@/lib/utils'
import { toast } from '@/composables/useToast'
import type { Instance } from '@/views/configs/types'

// 配置详情「实例列表」标签页。
// 支持 500Mbps NLB 实际公网访问 IP 优先展示、端口映射标注、以及安全复制 SSH 命令。
defineProps<{
  instances: Instance[]
  loading: boolean
  actionLoading: Record<string, boolean>
}>()

const emit = defineEmits<{
  control: [instanceId: string, action: string]
  terminate: [instanceId: string]
  'change-ip': [instanceId: string]
  edit: [instance: Instance]
  'cloud-shell': [instanceId: string]
}>()

const copyText = async (text: string, label: string) => {
  try {
    await copyToClipboard(text)
    toast.success(`${label}已复制`)
  } catch {
    toast.error('复制失败，请手动复制')
  }
}

const copySsh = async (instance: Instance) => {
  const bw = instance.bandwidth500
  if (bw?.enabled && bw.accessIp) {
    let port = 22
    if (bw.endpoints && bw.endpoints.length > 0) {
      const sshEp = bw.endpoints.find(e => e.id === 'ssh' || e.backendPort === 22) || bw.endpoints[0]
      if (sshEp && sshEp.listenPort > 0) {
        port = sshEp.listenPort
      }
    }
    const cmd = port === 22 ? `ssh root@${bw.accessIp}` : `ssh -p ${port} root@${bw.accessIp}`
    await copyText(cmd, 'SSH 命令')
    return
  }

  const primaryIp = instance.publicIps?.[0]
  if (primaryIp) {
    await copyText(`ssh root@${primaryIp}`, 'SSH 命令')
  } else {
    toast.warning('实例暂无可用公网 IP')
  }
}

const handleChangeIpClick = (instance: Instance) => {
  if (instance.bandwidth500?.enabled) {
    if (
      !confirm(
        '提示：当前实例已启用 500Mbps，主访问入口为负载均衡器(NLB)公网地址。更改此处的网卡IP不会改变 NLB 的访问入口，确定继续更改实例网卡IP吗？'
      )
    ) {
      return
    }
  }
  emit('change-ip', instance.id)
}
</script>

<template>
  <div v-if="loading" class="flex items-center justify-center py-12">
    <Loader2 class="w-10 h-10 animate-spin text-primary" />
  </div>
  <div v-else-if="!instances.length" class="text-center py-16">
    <div class="w-20 h-20 mx-auto mb-4 rounded-full bg-muted/50 flex items-center justify-center">
      <Server class="w-10 h-10 text-muted-foreground" />
    </div>
    <p class="text-muted-foreground text-lg">暂无实例</p>
  </div>
  <div v-else class="grid grid-cols-1 xl:grid-cols-2 gap-4">
    <Card v-for="instance in instances" :key="instance.id" class="p-5 hover:border-primary/50 transition-colors">
      <div class="flex justify-between items-start mb-4">
        <div class="flex-1 min-w-0 pr-4">
          <h4 class="font-semibold text-lg truncate">{{ instance.displayName }}</h4>
          <p class="text-xs text-muted-foreground font-mono truncate mt-1">{{ instance.id }}</p>
        </div>
        <Badge
          :variant="instance.state === 'RUNNING' ? 'success' : instance.state === 'STOPPED' ? 'destructive' : 'warning'"
          class="shrink-0"
        >
          {{ instance.state }}
        </Badge>
      </div>
      <div class="grid grid-cols-2 gap-3 text-sm mb-5">
        <div class="bg-muted/30 rounded-lg p-3">
          <span class="text-muted-foreground text-xs block mb-1">规格</span>
          <span class="font-medium">{{ instance.shape }}</span>
        </div>
        <div class="bg-muted/30 rounded-lg p-3">
          <span class="text-muted-foreground text-xs block mb-1">CPU / 内存</span>
          <span class="font-medium">{{ instance.ocpus }}核 / {{ instance.memory }}GB</span>
        </div>
        <div class="bg-muted/30 rounded-lg p-3">
          <span class="text-muted-foreground text-xs block mb-1">引导卷</span>
          <span class="font-medium">{{ instance.bootVolumeSize || '-' }} GB</span>
        </div>
        <div class="bg-muted/30 rounded-lg p-3">
          <span class="text-muted-foreground text-xs block mb-1">区域</span>
          <span class="font-medium">{{ instance.region }}</span>
        </div>
        <!-- 500Mbps 启用状态卡片（优先展示 NLB 实际访问 IP） -->
        <div
          v-if="instance.bandwidth500?.enabled && instance.bandwidth500?.accessIp"
          class="bg-primary/5 border border-primary/30 rounded-lg p-3 col-span-2 space-y-2"
        >
          <div class="flex items-center justify-between">
            <span class="text-primary text-xs font-semibold flex items-center gap-1.5">
              <Zap class="w-3.5 h-3.5 fill-primary" />
              访问 IP (500Mbps / NLB)
            </span>
            <Badge variant="success" class="text-[11px] py-0 px-2 h-5">
              {{ instance.bandwidth500.mode === 'shared-ports' ? '共享 NLB' : '独享 NLB' }}
            </Badge>
          </div>
          <div class="flex items-center justify-between gap-2">
            <span class="font-mono text-base font-bold text-primary tracking-wide">
              {{ instance.bandwidth500.accessIp }}
            </span>
            <div class="flex items-center gap-1.5 shrink-0">
              <Button
                size="sm"
                variant="outline"
                class="h-7 text-xs px-2"
                @click="copyText(instance.bandwidth500.accessIp, 'NLB IP')"
              >
                <Copy class="w-3 h-3 mr-1" />
                复制IP
              </Button>
              <Button size="sm" variant="outline" class="h-7 text-xs px-2" @click="copySsh(instance)">
                <Terminal class="w-3 h-3 mr-1" />
                复制SSH
              </Button>
            </div>
          </div>
          <!-- 共享模式端口映射标注 -->
          <div
            v-if="instance.bandwidth500.mode === 'shared-ports' && instance.bandwidth500.endpoints?.length"
            class="flex flex-wrap gap-1.5 pt-1 text-xs"
          >
            <span class="text-muted-foreground">端口映射:</span>
            <span
              v-for="ep in instance.bandwidth500.endpoints"
              :key="ep.id"
              class="font-mono bg-background/80 px-1.5 py-0.5 rounded border border-border text-foreground"
            >
              {{ ep.protocol }} {{ ep.listenPort }} &rarr; {{ ep.backendPort }}
            </span>
          </div>
          <div v-else class="text-xs text-muted-foreground pt-0.5">TCP/UDP 全端口透明转发 (独享模式)</div>
          <!-- 标注实例原网卡公网 IP -->
          <div
            class="text-[11px] text-muted-foreground/80 border-t border-border/50 pt-1.5 flex items-center justify-between"
          >
            <span>实例原公网 IP (非当前推荐入口): {{ instance.publicIps?.join(', ') || '无' }}</span>
          </div>
        </div>

        <!-- 500Mbps 启用但地址获取中/待核验 -->
        <div
          v-else-if="instance.bandwidth500?.enabled"
          class="bg-warning/10 border border-warning/30 rounded-lg p-3 col-span-2 space-y-1.5"
        >
          <div class="flex items-center justify-between text-warning text-xs font-semibold">
            <span class="flex items-center gap-1.5">
              <Loader2 class="w-3.5 h-3.5 animate-spin" />
              500Mbps 访问地址获取中 / 待核验
            </span>
            <Badge variant="warning" class="text-[11px] py-0 px-2 h-5">已启用</Badge>
          </div>
          <p class="text-xs text-muted-foreground">当前配置已生效，正在核验外部 NLB 访问地址，请稍候刷新。</p>
          <div class="text-[11px] text-muted-foreground">
            实例原公网 IP: {{ instance.publicIps?.join(', ') || '无' }}
          </div>
        </div>

        <!-- 未启用 500Mbps（正常展示实例公网 IP） -->
        <div v-else class="bg-muted/30 rounded-lg p-3 col-span-2">
          <div class="flex items-center justify-between mb-1">
            <span class="text-muted-foreground text-xs block">实例公网IP</span>
            <Button
              v-if="instance.publicIps?.[0]"
              size="sm"
              variant="ghost"
              class="h-6 text-xs px-1.5 text-muted-foreground hover:text-foreground"
              @click="copySsh(instance)"
            >
              <Terminal class="w-3 h-3 mr-1" />
              复制SSH
            </Button>
          </div>
          <span class="font-mono text-sm font-medium">{{ instance.publicIps?.join(', ') || '无' }}</span>
        </div>
        <div v-if="instance.ipv6" class="bg-primary/5 border border-primary/20 rounded-lg p-3 col-span-2">
          <span class="text-primary text-xs block mb-1">IPv6</span>
          <span class="font-mono text-sm text-primary">{{ instance.ipv6 }}</span>
        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button
          size="sm"
          variant="success"
          :disabled="instance.state === 'RUNNING' || actionLoading[instance.id]"
          @click="$emit('control', instance.id, 'START')"
        >
          <Play class="w-3.5 h-3.5" />
          启动
        </Button>
        <Button
          size="sm"
          variant="warning"
          :disabled="instance.state !== 'RUNNING' || actionLoading[instance.id]"
          @click="$emit('control', instance.id, 'STOP')"
        >
          <Square class="w-3.5 h-3.5" />
          停止
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="instance.state !== 'RUNNING' || actionLoading[instance.id]"
          @click="$emit('control', instance.id, 'SOFTRESET')"
        >
          <RotateCcw class="w-3.5 h-3.5" />
          重启
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="actionLoading[instance.id]"
          @click="handleChangeIpClick(instance)"
        >
          <Globe class="w-3.5 h-3.5" />
          更改IP
        </Button>
        <Button size="sm" variant="outline" @click="$emit('edit', instance)">
          <Settings class="w-3.5 h-3.5" />
          编辑配置
        </Button>
        <Button size="sm" variant="outline" @click="$emit('cloud-shell', instance.id)">
          <Terminal class="w-3.5 h-3.5" />
          Cloud Shell
        </Button>
        <Button
          size="sm"
          variant="destructive"
          :disabled="actionLoading[instance.id]"
          @click="$emit('terminate', instance.id)"
        >
          <Trash2 class="w-3.5 h-3.5" />
          删除
        </Button>
      </div>
    </Card>
  </div>
</template>
