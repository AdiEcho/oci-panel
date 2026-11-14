<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  Network,
  Plus,
  Search,
  Edit,
  Trash2,
  Loader2,
  ChevronLeft,
  ChevronRight,
  Zap,
  Upload,
  CheckCircle2,
  XCircle
} from '@lucide/vue'
import { proxyApi, type ProxyItem, type ProxyTestResult, type ProxyImportResult } from '@/api'
import { toast } from '@/composables/useToast'
import { usePagination } from '@/composables/usePagination'
import { useSelection } from '@/composables/useSelection'
import { debounce } from '@/lib/utils'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'
import { Select } from '@/components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import { Dialog, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'

const proxies = ref<ProxyItem[]>([])
const loading = ref(false)
const searchText = ref('')
const filterProtocol = ref('')
const { currentPage, pageSize, totalPages, applyResult } = usePagination()
const {
  selectedIds,
  isAllSelected,
  isIndeterminate,
  toggle: toggleSelect,
  toggleAll: toggleSelectAll
} = useSelection(proxies, p => p.id)

const showEditModal = ref(false)
const editingProxy = ref<ProxyItem | null>(null)
const submitting = ref(false)
const testing = ref(false)
const testResult = ref<ProxyTestResult | null>(null)
const testingId = ref('')
const batchTesting = ref(false)
const probeResults = ref<Record<string, ProxyTestResult>>({})
const showImportModal = ref(false)
const importText = ref('')
const importResult = ref<ProxyImportResult | null>(null)
const importing = ref(false)

const form = ref({
  name: '',
  protocol: 'http',
  host: '',
  port: 8080,
  username: '',
  password: '',
  enabled: true,
  remark: ''
})

const protocolLabels: Record<string, string> = {
  http: 'HTTP',
  https: 'HTTPS',
  socks5: 'SOCKS5'
}

const loadProxies = async (page = 1) => {
  loading.value = true
  try {
    const response = await proxyApi.page({
      page,
      pageSize,
      name: searchText.value,
      protocol: filterProtocol.value
    })
    proxies.value = response.data.list || []
    applyResult(response.data)
  } catch (error: any) {
    toast.error(error.message || '加载失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = debounce(() => loadProxies(1), 300)

const openCreate = () => {
  editingProxy.value = null
  testResult.value = null
  form.value = {
    name: '',
    protocol: 'http',
    host: '',
    port: 8080,
    username: '',
    password: '',
    enabled: true,
    remark: ''
  }
  showEditModal.value = true
}

const editProxy = (proxy: ProxyItem) => {
  editingProxy.value = proxy
  testResult.value = null
  form.value = {
    name: proxy.name,
    protocol: proxy.protocol,
    host: proxy.host,
    port: proxy.port,
    username: proxy.username,
    password: '',
    enabled: proxy.enabled,
    remark: proxy.remark || ''
  }
  showEditModal.value = true
}

const closeModal = () => {
  showEditModal.value = false
  editingProxy.value = null
  testResult.value = null
}

const submitForm = async () => {
  if (!form.value.name.trim() || !form.value.host.trim()) {
    toast.warning('请填写代理名称与地址')
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: form.value.name.trim(),
      protocol: form.value.protocol,
      host: form.value.host.trim(),
      port: Number(form.value.port),
      username: form.value.username,
      // 编辑时密码留空表示保持原值，交由后端处理。
      password: form.value.password || undefined,
      enabled: form.value.enabled,
      remark: form.value.remark
    }
    if (editingProxy.value) {
      await proxyApi.update({ id: editingProxy.value.id, ...payload })
      toast.success('代理已更新')
    } else {
      await proxyApi.create(payload)
      toast.success('代理已添加')
    }
    closeModal()
    await loadProxies(currentPage.value)
  } catch (error: any) {
    toast.error(error.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const testForm = async () => {
  if (!form.value.host.trim()) {
    toast.warning('请先填写代理地址')
    return
  }
  testing.value = true
  testResult.value = null
  try {
    const response = await proxyApi.test({
      id: editingProxy.value?.id,
      protocol: form.value.protocol,
      host: form.value.host.trim(),
      port: Number(form.value.port),
      username: form.value.username,
      password: form.value.password || undefined
    })
    testResult.value = response.data
    if (response.data.reachable) {
      toast.success(`连接成功，出口 IP：${response.data.egressIp || '未知'}`)
    } else {
      toast.error(response.data.message || '连接失败')
    }
  } catch (error: any) {
    toast.error(error.message || '测试失败')
  } finally {
    testing.value = false
  }
}

const testSaved = async (proxy: ProxyItem) => {
  testingId.value = proxy.id
  try {
    const response = await proxyApi.test({ id: proxy.id })
    probeResults.value[proxy.id] = response.data
    if (response.data.reachable) {
      toast.success(`${proxy.name} 连接成功，延迟 ${response.data.latencyMs}ms`)
    } else {
      toast.error(`${proxy.name}：${response.data.message || '连接失败'}`)
    }
  } catch (error: any) {
    toast.error(error.message || '测试失败')
  } finally {
    testingId.value = ''
  }
}

const runBatchTest = async () => {
  const targets = selectedIds.value.length ? [...selectedIds.value] : proxies.value.map(proxy => proxy.id)
  if (!targets.length) {
    toast.warning('当前没有可探测的代理')
    return
  }
  batchTesting.value = true
  try {
    const response = await proxyApi.batchTest(targets)
    let reachable = 0
    for (const item of response.data) {
      probeResults.value[item.id] = item.result
      if (item.result.reachable) reachable++
    }
    if (reachable === response.data.length) {
      toast.success(`探测完成：${reachable}/${response.data.length} 个代理可用`)
    } else {
      toast.warning(`探测完成：${reachable}/${response.data.length} 个代理可用`)
    }
  } catch (error: any) {
    toast.error(error.message || '批量探测失败')
  } finally {
    batchTesting.value = false
  }
}

const openImport = () => {
  importText.value = ''
  importResult.value = null
  showImportModal.value = true
}

const submitImport = async () => {
  if (!importText.value.trim()) {
    toast.warning('请粘贴代理列表')
    return
  }
  importing.value = true
  try {
    const response = await proxyApi.importText(importText.value)
    importResult.value = response.data
    if (response.data.imported > 0) {
      toast.success(`成功导入 ${response.data.imported} 个代理`)
      await loadProxies(1)
    } else {
      toast.warning('没有新增代理，请检查重复或格式')
    }
  } catch (error: any) {
    toast.error(error.message || '导入失败')
  } finally {
    importing.value = false
  }
}

const deleteProxy = async (proxy: ProxyItem) => {
  const suffix = proxy.bindCount > 0 ? `，${proxy.bindCount} 个配置将改为直连` : ''
  if (!confirm(`确定要删除代理「${proxy.name}」吗${suffix}？`)) return
  try {
    await proxyApi.delete([proxy.id])
    toast.success('代理已删除')
    await loadProxies(currentPage.value)
  } catch (error: any) {
    toast.error(error.message || '删除失败')
  }
}

onMounted(() => {
  loadProxies()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div
      v-motion
      :initial="{ opacity: 0, y: -20 }"
      :enter="{ opacity: 1, y: 0 }"
      class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4"
    >
      <div class="flex items-center gap-4">
        <h1 class="text-3xl font-display font-bold">代理管理</h1>
        <Badge variant="secondary">{{ proxies.length }} 个</Badge>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <Button variant="outline" :disabled="batchTesting || !proxies.length" @click="runBatchTest">
          <Loader2 v-if="batchTesting" class="w-4 h-4 animate-spin" />
          <Zap v-else class="w-4 h-4" />
          {{ batchTesting ? '探测中...' : '批量探测' }}
        </Button>
        <Button variant="outline" @click="openImport">
          <Upload class="w-4 h-4" />
          批量导入
        </Button>
        <Button @click="openCreate">
          <Plus class="w-4 h-4" />
          添加代理
        </Button>
      </div>
    </div>

    <Card
      v-motion
      :initial="{ opacity: 0, y: 20 }"
      :enter="{ opacity: 1, y: 0, transition: { delay: 100 } }"
      class="border-border/50"
    >
      <CardHeader class="border-b border-border/50 pb-4">
        <div class="flex gap-4">
          <div class="relative max-w-md flex-1">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input v-model="searchText" placeholder="搜索代理名称..." class="pl-10" @input="handleSearch" />
          </div>
          <Select v-model="filterProtocol" class="w-40" @change="loadProxies(1)">
            <option value="">全部协议</option>
            <option value="http">HTTP</option>
            <option value="https">HTTPS</option>
            <option value="socks5">SOCKS5</option>
          </Select>
        </div>
      </CardHeader>

      <CardContent class="p-0">
        <Table>
          <TableHeader>
            <TableRow class="hover:bg-transparent">
              <TableHead class="w-12">
                <Checkbox
                  :model-value="isAllSelected"
                  :indeterminate="isIndeterminate"
                  @update:model-value="toggleSelectAll"
                />
              </TableHead>
              <TableHead>名称</TableHead>
              <TableHead>协议</TableHead>
              <TableHead>地址</TableHead>
              <TableHead>认证</TableHead>
              <TableHead>绑定配置</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>探测</TableHead>
              <TableHead class="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-if="loading">
              <TableCell colspan="9" class="h-32 text-center">
                <Loader2 class="w-8 h-8 mx-auto animate-spin text-primary" />
              </TableCell>
            </TableRow>
            <TableRow v-else-if="!proxies.length">
              <TableCell colspan="9" class="h-32 text-center text-muted-foreground">
                暂无代理，点击右上角添加 HTTP 或 SOCKS5 代理
              </TableCell>
            </TableRow>
            <TableRow
              v-for="(proxy, index) in proxies"
              v-else
              :key="proxy.id"
              v-motion
              :initial="{ opacity: 0, x: -20 }"
              :enter="{ opacity: 1, x: 0, transition: { delay: 50 * index } }"
              class="group"
            >
              <TableCell>
                <Checkbox :model-value="selectedIds.includes(proxy.id)" @update:model-value="toggleSelect(proxy.id)" />
              </TableCell>
              <TableCell class="font-medium">
                <div class="flex items-center gap-2">
                  <div class="w-8 h-8 rounded bg-primary/10 flex items-center justify-center">
                    <Network class="w-4 h-4 text-primary" />
                  </div>
                  <div>
                    <div>{{ proxy.name }}</div>
                    <div v-if="proxy.remark" class="text-xs text-muted-foreground font-normal">
                      {{ proxy.remark }}
                    </div>
                  </div>
                </div>
              </TableCell>
              <TableCell>
                <Badge variant="info">{{ protocolLabels[proxy.protocol] || proxy.protocol }}</Badge>
              </TableCell>
              <TableCell class="font-mono text-sm">{{ proxy.host }}:{{ proxy.port }}</TableCell>
              <TableCell class="text-muted-foreground text-sm">
                {{ proxy.hasAuth ? proxy.username || '已配置' : '无' }}
              </TableCell>
              <TableCell>
                <Badge :variant="proxy.bindCount > 0 ? 'success' : 'secondary'">{{ proxy.bindCount }}</Badge>
              </TableCell>
              <TableCell>
                <Badge :variant="proxy.enabled ? 'success' : 'secondary'">
                  {{ proxy.enabled ? '启用' : '禁用' }}
                </Badge>
              </TableCell>
              <TableCell>
                <Badge
                  v-if="probeResults[proxy.id]"
                  :variant="probeResults[proxy.id].reachable ? 'success' : 'destructive'"
                  :title="probeResults[proxy.id].message"
                >
                  {{ probeResults[proxy.id].reachable ? `${probeResults[proxy.id].latencyMs}ms` : '失败' }}
                </Badge>
                <span v-else class="text-xs text-muted-foreground">-</span>
              </TableCell>
              <TableCell class="text-right">
                <div class="flex justify-end items-center gap-1">
                  <Button variant="outline" size="sm" :disabled="testingId === proxy.id" @click="testSaved(proxy)">
                    <Loader2 v-if="testingId === proxy.id" class="w-3.5 h-3.5 animate-spin" />
                    <Zap v-else class="w-3.5 h-3.5" />
                    <span class="hidden sm:inline ml-1">测试</span>
                  </Button>
                  <Button variant="ghost" size="icon" title="编辑" @click="editProxy(proxy)">
                    <Edit class="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    title="删除"
                    class="text-destructive hover:text-destructive"
                    @click="deleteProxy(proxy)"
                  >
                    <Trash2 class="w-4 h-4" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>

        <div v-if="totalPages > 1" class="flex items-center justify-center gap-2 p-4 border-t border-border/50">
          <Button variant="outline" size="sm" :disabled="currentPage === 1" @click="loadProxies(currentPage - 1)">
            <ChevronLeft class="w-4 h-4" />
            上一页
          </Button>
          <span class="px-4 text-sm text-muted-foreground">第 {{ currentPage }} / {{ totalPages }} 页</span>
          <Button
            variant="outline"
            size="sm"
            :disabled="currentPage === totalPages"
            @click="loadProxies(currentPage + 1)"
          >
            下一页
            <ChevronRight class="w-4 h-4" />
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Import Modal -->
    <Dialog v-model:open="showImportModal">
      <DialogHeader class="mb-4">
        <DialogTitle>批量导入代理</DialogTitle>
        <DialogDescription>每行一个代理，重复代理自动跳过，格式错误会按行提示</DialogDescription>
      </DialogHeader>

      <form class="space-y-4" @submit.prevent="submitImport">
        <div>
          <label class="block text-sm font-medium mb-2">代理列表</label>
          <Textarea
            v-model="importText"
            :rows="8"
            class="font-mono text-sm"
            placeholder="1.2.3.4:8080&#10;5.6.7.8:1080:user:pass&#10;socks5://user:pass@9.9.9.9:1080#新加坡&#10;https://proxy.example.com:8443"
            required
          />
          <div class="text-xs text-muted-foreground mt-2 space-y-1">
            <p>支持 host:port、host:port:user:pass、http/https/socks5 URL 三种写法</p>
            <p>行尾 #名称 可指定代理名称；# 开头的整行视为注释</p>
          </div>
        </div>

        <div v-if="importResult" class="rounded-md border border-border/60 p-3 text-sm space-y-2">
          <div class="flex flex-wrap gap-4">
            <span class="text-success">导入 {{ importResult.imported }}</span>
            <span class="text-muted-foreground">跳过 {{ importResult.skipped }}</span>
            <span :class="importResult.failed ? 'text-destructive' : 'text-muted-foreground'">
              失败 {{ importResult.failed }}
            </span>
          </div>
          <ul v-if="importResult.errors.length" class="text-xs text-destructive space-y-1 max-h-32 overflow-y-auto">
            <li v-for="(error, index) in importResult.errors" :key="index">{{ error }}</li>
          </ul>
        </div>

        <DialogFooter class="mt-6">
          <Button type="button" variant="outline" @click="showImportModal = false">关闭</Button>
          <Button type="submit" :disabled="importing">
            <Loader2 v-if="importing" class="w-4 h-4 animate-spin" />
            {{ importing ? '导入中...' : '开始导入' }}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>

    <!-- Add/Edit Modal -->
    <Dialog v-model:open="showEditModal">
      <DialogHeader class="mb-4">
        <DialogTitle>{{ editingProxy ? '编辑代理' : '添加代理' }}</DialogTitle>
        <DialogDescription>支持 HTTP、HTTPS 与 SOCKS5 代理，绑定后 OCI 请求经代理转发</DialogDescription>
      </DialogHeader>

      <form class="space-y-4" @submit.prevent="submitForm">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium mb-2">代理名称</label>
            <Input v-model="form.name" placeholder="例: 新加坡出口" required />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">协议</label>
            <Select v-model="form.protocol">
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
              <option value="socks5">SOCKS5</option>
            </Select>
          </div>
        </div>

        <div class="grid grid-cols-3 gap-4">
          <div class="col-span-2">
            <label class="block text-sm font-medium mb-2">代理地址</label>
            <Input v-model="form.host" placeholder="127.0.0.1 或 proxy.example.com" required />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">端口</label>
            <Input v-model.number="form.port" type="number" min="1" max="65535" required />
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium mb-2">用户名</label>
            <Input v-model="form.username" placeholder="可选" autocomplete="off" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-2">密码</label>
            <Input
              v-model="form.password"
              type="password"
              :placeholder="editingProxy ? '留空表示不修改' : '可选'"
              autocomplete="new-password"
            />
          </div>
        </div>

        <div>
          <label class="block text-sm font-medium mb-2">备注</label>
          <Textarea v-model="form.remark" :rows="2" placeholder="可选，例：仅用于 ap-singapore-1" />
        </div>

        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <Switch v-model="form.enabled" />
            <span class="text-sm text-muted-foreground">{{ form.enabled ? '启用' : '禁用' }}</span>
          </div>
          <Button type="button" variant="outline" :disabled="testing" @click="testForm">
            <Loader2 v-if="testing" class="w-4 h-4 animate-spin" />
            <Zap v-else class="w-4 h-4" />
            {{ testing ? '测试中...' : '测试连接' }}
          </Button>
        </div>

        <div
          v-if="testResult"
          :class="[
            'flex items-start gap-2 rounded-md border p-3 text-sm',
            testResult.reachable ? 'border-success/40 bg-success/5' : 'border-destructive/40 bg-destructive/5'
          ]"
        >
          <CheckCircle2 v-if="testResult.reachable" class="w-4 h-4 text-success mt-0.5" />
          <XCircle v-else class="w-4 h-4 text-destructive mt-0.5" />
          <div>
            <p>{{ testResult.message }}</p>
            <p v-if="testResult.reachable" class="text-xs text-muted-foreground mt-1">
              延迟 {{ testResult.latencyMs }}ms
              <span v-if="testResult.egressIp">· 出口 IP {{ testResult.egressIp }}</span>
            </p>
          </div>
        </div>

        <DialogFooter class="mt-6">
          <Button type="button" variant="outline" @click="closeModal">取消</Button>
          <Button type="submit" :disabled="submitting">
            <Loader2 v-if="submitting" class="w-4 h-4 animate-spin" />
            {{ submitting ? '提交中...' : '提交' }}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  </div>
</template>
