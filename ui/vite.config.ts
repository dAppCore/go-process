import { defineConfig } from 'vite';
import { resolve } from 'path';

export default defineConfig({
  build: {
    lib: {
      entry: resolve(__dirname, 'src/index.ts'),
      name: 'CoreProcess',
      fileName: 'core-process',
      formats: ['es'],
    },
    outDir: resolve(__dirname, '../pkg/api/ui/dist'),
    emptyOutDir: true,
    rollupOptions: {
      output: {
        entryFileNames: 'core-process.js',
      },
    },
  },
});
