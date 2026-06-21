import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      output: {
        onlyExplicitManualChunks: true,
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return undefined;
          }
          if (id.includes("/react/") || id.includes("/react-dom/") || id.includes("/react-router-dom/")) {
            return "vendor-react";
          }
          if (
            id.includes("/rc-") ||
            id.includes("/@rc-component/") ||
            id.includes("/@ant-design/cssinjs") ||
            id.includes("/@ant-design/colors/") ||
            id.includes("/@ant-design/fast-color/")
          ) {
            return "vendor-antd-rc";
          }
          if (id.includes("/antd/") || id.includes("/@ant-design/")) {
            return "vendor-antd";
          }
          if (id.includes("/echarts/") || id.includes("/echarts-for-react/") || id.includes("/zrender/")) {
            return "vendor-echarts";
          }
          if (id.includes("/klinecharts/")) {
            return "vendor-klinecharts";
          }
          if (id.includes("/@lobehub/icons/")) {
            return "vendor-lobe-icons";
          }
          return undefined;
        },
      },
    },
  },
  server: {
    host: "127.0.0.1",
    port: 1420,
    strictPort: true
  },
  clearScreen: false
});
