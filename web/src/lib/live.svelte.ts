import type { LiveMonitor } from './types.ts';

type Handlers = {
	monitor?: (event: LiveMonitor) => void;
	layout?: (event: LiveMonitor) => void;
	incident?: () => void;
};

export class Live {
	connected = $state(false);

	constructor(handlers: Handlers, admin = false) {
		$effect(() => {
			const source = new EventSource(admin ? '/api/stream?admin' : '/api/stream');

			source.onopen = () => (this.connected = true);
			source.onerror = () => (this.connected = false);

			for (const type of ['monitor', 'layout'] as const) {
				source.addEventListener(type, (e) => handlers[type]?.(JSON.parse(e.data)));
			}
			source.addEventListener('incident', () => handlers.incident?.());

			return () => {
				source.close();
				this.connected = false;
			};
		});
	}
}

export function debounce(fn: () => void, wait = 400) {
	let timer: ReturnType<typeof setTimeout> | undefined;
	return () => {
		clearTimeout(timer);
		timer = setTimeout(fn, wait);
	};
}

export class Clock {
	now = $state(Date.now());

	constructor(every = 15_000) {
		$effect(() => {
			const id = setInterval(() => (this.now = Date.now()), every);
			return () => clearInterval(id);
		});
	}
}
