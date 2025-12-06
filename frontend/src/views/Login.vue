<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMotion } from '@vueuse/motion'
import { Cloud, Lock, User, ArrowRight, Loader2, Shield, Fingerprint } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { toast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import api from '@/lib/api'

const router = useRouter()
const authStore = useAuthStore()

const form = ref({
  account: '',
  password: ''
})

const loading = ref(false)
const error = ref('')
const needMfa = ref(false)
const needPasskey = ref(false)
const passkeyEnabled = ref(false)
const mfaCode = ref('')
const verifyingMfa = ref(false)
const verifyingPasskey = ref(false)

const cardRef = ref<HTMLElement>()

useMotion(cardRef, {
  initial: { opacity: 0, y: 50, scale: 0.95 },
  enter: {
    opacity: 1,
    y: 0,
    scale: 1,
    transition: {
      duration: 600,
      ease: [0.16, 1, 0.3, 1]
    }
  }
})

const handleLogin = async () => {
  error.value = ''
  loading.value = true

  try {
    const result = await authStore.login(form.value.account, form.value.password)
    if (result.needMfa || result.needPasskey) {
      needMfa.value = result.needMfa
      needPasskey.value = result.needPasskey
      passkeyEnabled.value = result.passkeyEnabled
      loading.value = false
      return
    }
    toast.success('登录成功')
    router.push('/')
  } catch (err: any) {
    error.value = err.message || '登录失败，请检查账号密码'
    toast.error(error.value)
  } finally {
    loading.value = false
  }
}

const handleMfaVerify = async () => {
  if (!mfaCode.value || mfaCode.value.length !== 6) {
    error.value = '请输入6位验证码'
    return
  }
  error.value = ''
  verifyingMfa.value = true

  try {
    await authStore.verifyMfa(mfaCode.value)
    toast.success('登录成功')
    router.push('/')
  } catch (err: any) {
    error.value = err.message || '验证码错误'
    toast.error(error.value)
  } finally {
    verifyingMfa.value = false
  }
}

const handlePasskeyLogin = async () => {
  error.value = ''
  verifyingPasskey.value = true

  try {
    const beginResponse = await api.post('/passkey/beginLogin', {})
    const options = beginResponse.data

    const credential = (await navigator.credentials.get({
      publicKey: {
        ...options.publicKey,
        challenge: base64ToArrayBuffer(options.publicKey.challenge),
        allowCredentials:
          options.publicKey.allowCredentials?.map((c: any) => ({
            ...c,
            id: base64ToArrayBuffer(c.id)
          })) || []
      }
    })) as PublicKeyCredential

    const assertionResponse = credential.response as AuthenticatorAssertionResponse
    const credentialData = {
      id: credential.id,
      rawId: arrayBufferToBase64(credential.rawId),
      type: credential.type,
      response: {
        clientDataJSON: arrayBufferToBase64(assertionResponse.clientDataJSON),
        authenticatorData: arrayBufferToBase64(assertionResponse.authenticatorData),
        signature: arrayBufferToBase64(assertionResponse.signature),
        userHandle: assertionResponse.userHandle ? arrayBufferToBase64(assertionResponse.userHandle) : null
      }
    }

    const finishResponse = await api.post('/passkey/finishLogin', { credential: credentialData })
    authStore.setToken(finishResponse.data.token, finishResponse.data.username)
    toast.success('登录成功')
    router.push('/')
  } catch (err: any) {
    if (err.name === 'NotAllowedError') {
      error.value = '用户取消了操作'
    } else {
      error.value = err.message || 'Passkey 验证失败'
    }
    toast.error(error.value)
  } finally {
    verifyingPasskey.value = false
  }
}

function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binaryString = atob(base64.replace(/-/g, '+').replace(/_/g, '/'))
  const bytes = new Uint8Array(binaryString.length)
  for (let i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i)
  }
  return bytes.buffer
}

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
}

const backToLogin = () => {
  needMfa.value = false
  needPasskey.value = false
  passkeyEnabled.value = false
  mfaCode.value = ''
  error.value = ''
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center p-4 relative overflow-hidden">
    <!-- Animated Background -->
    <div class="absolute inset-0 mesh-gradient" />
    <div class="absolute inset-0 grid-pattern opacity-30" />
    <div class="absolute inset-0 noise-overlay" />

    <!-- Floating Elements -->
    <div class="absolute top-1/4 left-1/4 w-64 h-64 bg-primary/10 rounded-full blur-3xl animate-pulse" />
    <div class="absolute bottom-1/4 right-1/4 w-96 h-96 bg-cyan-500/5 rounded-full blur-3xl animate-pulse delay-1000" />

    <!-- Login Card -->
    <Card ref="cardRef" class="relative z-10 w-full max-w-md glass border-border/50">
      <CardContent class="p-8">
        <!-- Logo -->
        <div class="text-center mb-8">
          <div
            v-motion
            :initial="{ scale: 0, rotate: -180 }"
            :enter="{ scale: 1, rotate: 0, transition: { delay: 200, duration: 500 } }"
            class="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-primary to-cyan-400 rounded-2xl mb-4 shadow-lg"
          >
            <Cloud class="w-8 h-8 text-white" />
          </div>
          <h1
            v-motion
            :initial="{ opacity: 0, y: 20 }"
            :enter="{ opacity: 1, y: 0, transition: { delay: 300 } }"
            class="text-3xl font-display font-bold text-gradient"
          >
            OCI Panel
          </h1>
          <p
            v-motion
            :initial="{ opacity: 0 }"
            :enter="{ opacity: 1, transition: { delay: 400 } }"
            class="text-muted-foreground mt-2"
          >
            Oracle Cloud Infrastructure 管理面板
          </p>
        </div>

        <!-- Login Form -->
        <form v-if="!needMfa && !needPasskey" class="space-y-6" @submit.prevent="handleLogin">
          <div v-motion :initial="{ opacity: 0, x: -20 }" :enter="{ opacity: 1, x: 0, transition: { delay: 500 } }">
            <label class="block text-sm font-medium mb-2">
              <User class="w-4 h-4 inline mr-2 text-muted-foreground" />
              账号
            </label>
            <Input
              v-model="form.account"
              type="text"
              placeholder="请输入账号"
              required
              autocomplete="username"
              class="h-11 bg-secondary/50 border-border/50 focus:border-primary"
            />
          </div>

          <div v-motion :initial="{ opacity: 0, x: -20 }" :enter="{ opacity: 1, x: 0, transition: { delay: 600 } }">
            <label class="block text-sm font-medium mb-2">
              <Lock class="w-4 h-4 inline mr-2 text-muted-foreground" />
              密码
            </label>
            <Input
              v-model="form.password"
              type="password"
              placeholder="请输入密码"
              required
              autocomplete="current-password"
              class="h-11 bg-secondary/50 border-border/50 focus:border-primary"
            />
          </div>

          <div v-motion :initial="{ opacity: 0, y: 20 }" :enter="{ opacity: 1, y: 0, transition: { delay: 700 } }">
            <Button
              type="submit"
              :disabled="loading"
              :loading="loading"
              class="w-full h-11 text-base font-medium group"
            >
              <template v-if="!loading">
                登录
                <ArrowRight class="w-4 h-4 ml-2 transition-transform group-hover:translate-x-1" />
              </template>
              <template v-else>
                <Loader2 class="w-4 h-4 mr-2 animate-spin" />
                登录中...
              </template>
            </Button>
          </div>
        </form>

        <!-- MFA/Passkey Verification -->
        <div v-else class="space-y-6">
          <!-- Passkey Option -->
          <div v-if="passkeyEnabled" class="space-y-4">
            <div class="text-center mb-4">
              <div class="inline-flex items-center justify-center w-12 h-12 bg-primary/10 rounded-full mb-3">
                <Fingerprint class="w-6 h-6 text-primary" />
              </div>
              <p class="text-sm text-muted-foreground">使用 Passkey 快速登录</p>
            </div>

            <Button :disabled="verifyingPasskey" class="w-full h-11 text-base font-medium" @click="handlePasskeyLogin">
              <template v-if="!verifyingPasskey">
                <Fingerprint class="w-4 h-4 mr-2" />
                使用 Passkey 登录
              </template>
              <template v-else>
                <Loader2 class="w-4 h-4 mr-2 animate-spin" />
                验证中...
              </template>
            </Button>

            <div v-if="needMfa" class="relative">
              <div class="absolute inset-0 flex items-center">
                <span class="w-full border-t border-border" />
              </div>
              <div class="relative flex justify-center text-xs uppercase">
                <span class="bg-card px-2 text-muted-foreground">或</span>
              </div>
            </div>
          </div>

          <!-- MFA Option -->
          <form v-if="needMfa" class="space-y-4" @submit.prevent="handleMfaVerify">
            <div v-if="!passkeyEnabled" class="text-center mb-4">
              <div class="inline-flex items-center justify-center w-12 h-12 bg-primary/10 rounded-full mb-3">
                <Shield class="w-6 h-6 text-primary" />
              </div>
              <p class="text-sm text-muted-foreground">请输入验证器 App 中的验证码</p>
            </div>

            <div v-motion :initial="{ opacity: 0, y: 20 }" :enter="{ opacity: 1, y: 0 }">
              <Input
                v-model="mfaCode"
                type="text"
                placeholder="输入6位验证码"
                maxlength="6"
                :autofocus="!passkeyEnabled"
                class="h-12 text-center text-xl font-mono tracking-[0.5em] bg-secondary/50 border-border/50 focus:border-primary"
                @keyup.enter="handleMfaVerify"
              />
            </div>

            <Button
              type="submit"
              :disabled="verifyingMfa || mfaCode.length !== 6"
              class="w-full h-11 text-base font-medium"
            >
              <template v-if="!verifyingMfa">
                <Shield class="w-4 h-4 mr-2" />
                验证 MFA
              </template>
              <template v-else>
                <Loader2 class="w-4 h-4 mr-2 animate-spin" />
                验证中...
              </template>
            </Button>
          </form>

          <Button type="button" variant="ghost" class="w-full" @click="backToLogin">返回登录</Button>
        </div>

        <!-- Error Message -->
        <Transition
          enter-active-class="transition-all duration-300"
          leave-active-class="transition-all duration-300"
          enter-from-class="opacity-0 -translate-y-2"
          leave-to-class="opacity-0 -translate-y-2"
        >
          <div
            v-if="error"
            class="mt-4 p-3 bg-destructive/10 border border-destructive/30 rounded-lg text-destructive text-sm"
          >
            {{ error }}
          </div>
        </Transition>

        <!-- Footer -->
        <p
          v-motion
          :initial="{ opacity: 0 }"
          :enter="{ opacity: 1, transition: { delay: 800 } }"
          class="text-center text-xs text-muted-foreground mt-6"
        >
          安全登录 · 数据加密传输
        </p>
      </CardContent>
    </Card>
  </div>
</template>
