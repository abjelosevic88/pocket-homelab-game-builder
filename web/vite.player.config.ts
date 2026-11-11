// The player on its own (shared pages, games saved as one file): one script, one stylesheet.
import { defineConfig } from 'vite'

export default defineConfig({
  publicDir: false,
  build: {
    outDir: 'dist/play', emptyOutDir: true, sourcemap: false, cssCodeSplit: false,
    rollupOptions: {
      input: 'src/player/main.ts',
      output: { format: 'iife', entryFileNames: 'player.js', assetFileNames: 'player[extname]', inlineDynamicImports: true },
    },
  },
})
