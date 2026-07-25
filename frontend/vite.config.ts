import { defineConfig } from 'vite';

export default defineConfig({
  esbuild: {
    jsx: 'automatic',
    jsxImportSource: 'react',
  },
  build: {
    // Vite 4 ships an old esbuild that corrupts large render-chunk sourcemaps
    // in this project. Modern syntax plus Rollup output keeps production builds
    // deterministic until the toolchain is upgraded.
    target: 'esnext',
    minify: false,
    rollupOptions: {
      output: {
        manualChunks: {
          markdown: ['react-markdown', 'remark-gfm'],
        },
      },
    },
  },
});
