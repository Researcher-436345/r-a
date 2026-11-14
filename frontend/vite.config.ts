import { defineConfig, loadEnv } from 'vite';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', '');
  const settings: Record<string, string> = JSON.parse(execFileSync(
    'python3',
    ['-B', fileURLToPath(new URL('../settings.py', import.meta.url)), '--frontend'],
    { encoding: 'utf-8', env: { ...process.env, ...env } },
  ));
  const value = (name: string) => settings[name];

  return {
    define: Object.fromEntries(Object.keys(settings)
      .filter((name) => name.startsWith('VITE_'))
      .map((name) => [`import.meta.env.${name}`, JSON.stringify(value(name))])),
    esbuild: {
      jsx: 'automatic',
      jsxImportSource: 'react',
    },
    server: {
      port: Number(value('DEV_PORT')),
      strictPort: true,
      proxy: value('DEV_API_TARGET') ? {
        '/api': {
          target: value('DEV_API_TARGET'),
          changeOrigin: true,
          localAddress: value('DEV_API_LOCAL_ADDRESS') || undefined,
          rewrite: (path) => path.replace(/^\/api(?=\/|$)/, ''),
        },
      } : undefined,
    },
  };
});
