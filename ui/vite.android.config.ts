import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";

export default defineConfig({
  plugins: [react()],
  root: "ui",
  build: {
    outDir: "../dist/android",
    emptyOutDir: true,
    rollupOptions: { input: resolve("ui/android.html") },
  },
});
