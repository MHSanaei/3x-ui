import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'node:path';

// Output goes to web/dist/ at the repo root so the Go binary can embed it
// via embed.FS without reaching outside the web/ tree.
const outDir = path.resolve(__dirname, '../web/dist');

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir,
    emptyOutDir: true,
    sourcemap: true,
    target: 'es2020',
    // Multiple HTML entries — one per legacy page we migrate.
    // As pages get ported in later phases, add their entrypoints here.
    rollupOptions: {
      input: {
        index: path.resolve(__dirname, 'index.html'),
        login: path.resolve(__dirname, 'login.html'),
      },
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      // Proxy API calls during `npm run dev` to the local Go panel.
      '/panel': 'http://localhost:2053',
      '/server': 'http://localhost:2053',
      '/login': 'http://localhost:2053',
      '/logout': 'http://localhost:2053',
    },
  },
});
