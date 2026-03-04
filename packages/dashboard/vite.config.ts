import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

const proxyTarget = process.env.VITE_WARDEN_PROXY_TARGET || "http://127.0.0.1:7443";

// https://vite.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      "/api": {
        target: proxyTarget,
        changeOrigin: true,
        rewrite: (sourcePath) => sourcePath.replace(/^\/api/, ""),
      },
    },
  },
  plugins: [react(), tailwindcss()],
});
