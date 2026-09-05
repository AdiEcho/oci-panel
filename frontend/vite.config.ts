import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8999',
        changeOrigin: true
      },
      '/ws': {
        target: 'ws://localhost:8999',
        ws: true
      }
    }
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (
              id.includes('/node_modules/vue/') ||
              id.includes('/node_modules/@vue/') ||
              id.includes('/node_modules/pinia/') ||
              id.includes('/node_modules/vue-router/')
            ) {
              return 'vendor-vue'
            }
            if (id.includes('lucide') || id.includes('motion') || id.includes('radix')) {
              return 'vendor-ui'
            }
            return 'vendor'
          }
          if (id.includes('/components/ui/')) {
            return 'ui-components'
          }
        },
        chunkFileNames: 'assets/[name]-[hash].js',
        entryFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]'
      }
    },
    chunkSizeWarningLimit: 1000
  }
})
