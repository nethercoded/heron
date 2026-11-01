import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

const backend = process.env.HERON_BACKEND ?? 'http://localhost:8080';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				runes: ({ filename }) => (filename.split(/[/\\]/).includes('node_modules') ? undefined : true)
			},
			adapter: adapter({
				fallback: 'index.html',
				precompress: true
			}),
			version: {
				pollInterval: 0
			}
		})
	],
	server: {
		proxy: {
			'/api': { target: backend, changeOrigin: false },
			'/healthz': backend
		}
	}
});
