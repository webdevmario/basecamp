import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// In dev, scans, notes and the API come from `basecamp serve` (the launchd service on :3050).
// BASECAMP_API points the dev page at another server, e.g. a test build on another port.
const api = process.env.BASECAMP_API || 'http://localhost:3050';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/scan.json': api,
      '/jobs.json': api,
      '/api': api,
    },
  },
});
