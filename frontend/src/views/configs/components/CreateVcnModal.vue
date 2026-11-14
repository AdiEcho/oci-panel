<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { Loader2, Network } from '@lucide/vue'
import { vcnApi } from '@/api'
import { toast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Modal } from '@/components/ui/modal'
import { Switch } from '@/components/ui/switch'

// 新建 VCN 弹窗：与新建机器时的自动建网逻辑一致（VCN → Internet 网关 → 0.0.0.0/0 路由 → 公有子网）。
const props = defineProps<{
  open: boolean
  userId: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  refresh: []
}>()

const creating = ref(false)

const form = reactive({
  displayName: '',
  cidrBlock: '10.0.0.0/16',
  isIpv6Enabled: true
})

watch(
  () => props.open,
  open => {
    if (open) {
      form.displayName = `oci-panel-vcn-${Date.now().toString().slice(-6)}`
      form.cidrBlock = '10.0.0.0/16'
      form.isIpv6Enabled = true
    }
  }
)

const close = () => emit('update:open', false)

const submit = async () => {
  if (!form.displayName.trim()) {
    toast.warning('请输入VCN名称')
    return
  }
  if (!form.cidrBlock.trim()) {
    toast.warning('请输入CIDR地址')
    return
  }
  creating.value = true
  try {
    await vcnApi.create({
      configId: props.userId,
      displayName: form.displayName.trim(),
      cidrBlock: form.cidrBlock.trim(),
      isIpv6Enabled: form.isIpv6Enabled
    })
    toast.success('VCN创建成功')
    emit('refresh')
    close()
  } catch (error: any) {
    toast.error(error.message || '创建VCN失败')
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <Modal :open="open" max-width="max-w-lg" @update:open="emit('update:open', $event)">
    <template #title>
      <Network class="w-5 h-5 text-primary" />
      新建VCN
    </template>

    <div class="p-6 space-y-4">
      <div>
        <label class="block text-sm font-medium mb-2">VCN名称</label>
        <Input v-model="form.displayName" placeholder="oci-panel-vcn" />
      </div>

      <div>
        <label class="block text-sm font-medium mb-2">CIDR 地址块</label>
        <Input v-model="form.cidrBlock" placeholder="10.0.0.0/16" class="font-mono" />
        <p class="text-xs text-muted-foreground mt-2">用于VCN及配套公有子网，默认 10.0.0.0/16</p>
      </div>

      <div class="flex items-center justify-between rounded-lg border border-border p-4">
        <div>
          <p class="text-sm font-medium">启用 IPv6</p>
          <p class="text-xs text-muted-foreground mt-1">为VCN分配 IPv6 CIDR 块</p>
        </div>
        <Switch v-model="form.isIpv6Enabled" />
      </div>

      <div class="rounded-lg bg-muted/30 p-4 text-xs text-muted-foreground space-y-1">
        <p>将同时创建:</p>
        <p>- Internet 网关 {{ form.displayName || 'oci-panel-vcn' }}-igw</p>
        <p>- 默认路由 0.0.0.0/0 指向该网关</p>
        <p>- 公有子网 {{ form.displayName || 'oci-panel-vcn' }}-subnet（{{ form.cidrBlock || '10.0.0.0/16' }}）</p>
        <p>- 安全列表入站放行所有协议（含 UDP/ICMP）与全部端口</p>
      </div>
    </div>

    <div class="p-6 border-t border-border flex gap-3">
      <Button variant="outline" class="flex-1" :disabled="creating" @click="close">取消</Button>
      <Button class="flex-1" :disabled="creating" @click="submit">
        <Loader2 v-if="creating" class="w-4 h-4 animate-spin" />
        {{ creating ? '创建中...' : '创建' }}
      </Button>
    </div>
  </Modal>
</template>
