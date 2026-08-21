import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

const apiTarget = process.env.VITE_API_PROXY_TARGET ?? "http://127.0.0.1:8080";

export default defineConfig({
  plugins: [react()],
  cacheDir: process.env.VITE_CACHE_DIR ?? "node_modules/.vite",
  server: {
    port: 5173,
    // Same-origin /v1 → Go api.Handler() so the browser demo avoids CORS.
    // Containers override the target with the internal runtime service name.
    proxy: {
      "/v1": {
        target: apiTarget,
        changeOrigin: true,
      },
      "/auth": {
        target: apiTarget,
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
  },
});
