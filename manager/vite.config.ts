import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath } from 'node:url';

// The Go server serves the built files: index.html for /manager/* and the hashed
// files under /assets. Keep base '/' and outDir 'dist' so that contract holds.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const target = env.VITE_API_TARGET || 'http://localhost:8080';
  const apiPaths = ['/instance', '/send', '/license', '/swagger', '/health', '/server', '/user', '/chat', '/group', '/message', '/call'];

  return {
    base: '/',
    plugins: [react(), tailwindcss()],
    resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
    build: { outDir: 'dist', emptyOutDir: true, sourcemap: false, chunkSizeWarningLimit: 700 },
    server: {
      port: 5173,
      proxy: Object.fromEntries(apiPaths.map((p) => [p, { target, changeOrigin: true }])),
    },
    test: { environment: 'node', include: ['src/**/*.test.ts'] },
  };
});
