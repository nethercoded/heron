import type { LiveMonitor } from './types.ts';

type Handlers = {
	monitor?: (event: LiveMonitor) => void;
	layout?: (event: LiveMonitor) => void;
	incident?: () => void;
};

type Stream = { source: EventSource; open: boolean; listeners: Set<Live> };

const streams = new Map<string, Stream>();

function connect(url: string) {
	const existing = streams.get(url);
	if (existing) return existing;

	const stream: Stream = { source: new EventSource(url), open: false, listeners: new Set() };
	const each = (fn: (live: Live) => void) => {
		for (const live of [...stream.listeners]) fn(live);
	};
	stream.source.onopen = () => {
		stream.open = true;
		each((live) => (live.connected = true));
	};
	stream.source.onerror = () => {
		stream.open = false;
		each((live) => (live.connected = false));
	};
	for (const type of ['monitor', 'layout'] as const) {
		stream.source.addEventListener(type, (e) => {
			const event = JSON.parse(e.data);
			each((live) => live.handlers[type]?.(event));
		});
	}
	stream.source.addEventListener('incident', () => each((live) => live.handlers.incident?.()));
	streams.set(url, stream);
	return stream;
}

export class Live {
	connected = $state(false);
	readonly handlers: Handlers;

	constructor(handlers: Handlers, admin = false) {
		this.handlers = handlers;
		$effect(() => {
			const url = admin ? '/api/stream?admin' : '/api/stream';
			const stream = connect(url);
			stream.listeners.add(this);
			this.connected = stream.open;

			return () => {
				stream.listeners.delete(this);
				this.connected = false;
				if (stream.listeners.size === 0) {
					stream.source.close();
					streams.delete(url);
				}
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
