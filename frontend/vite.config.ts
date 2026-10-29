import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', '');

  return {
    esbuild: {
      jsx: 'automatic',
      jsxImportSource: 'react',
    },
    server: {
      proxy: env.DEV_API_TARGET ? {
        '/api': {
          target: env.DEV_API_TARGET,
          changeOrigin: true,
		  localAddress: env.DEV_API_LOCAL_ADDRESS || undefined,
          rewrite: (path) => path.replace(/^\/api(?=\/|$)/, ''),
        },
      } : undefined,
    },
  };
});
