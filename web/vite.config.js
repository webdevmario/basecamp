import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// In dev, scans, notes and the API come from `basecamp serve` (the launchd service on :3050).
const api = 'http://localhost:3050';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/scan.json': api,
      '/api': api,
    },
  },
});
