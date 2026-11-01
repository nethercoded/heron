import { createContext } from 'svelte';
import { api, message } from './api.ts';
import { debounce, Live } from './live.svelte.ts';
import type { LiveMonitor, StatusPage } from './types.ts';

export class StatusStore {
	data = $state<StatusPage | null>(null);
	error = $state<string | null>(null);
	live: Live;

	constructor(initial: StatusPage | null) {
		this.data = initial;
		const refresh = debounce(() => this.load(), 300);
		this.live = new Live({
			monitor: (event) => this.apply(event),
			layout: refresh,
			incident: refresh
		});
	}

	get page() {
		return this.data?.page ?? { title: 'Status' };
	}

	async load() {
		try {
			this.data = await api.get<StatusPage>('/status');
			this.error = null;
		} catch (err) {
			this.error = message(err);
		}
	}

	apply(event: LiveMonitor) {
		if (!this.data) return;
		for (const group of this.data.groups) {
			const m = group.monitors.find((m) => m.slug === event.slug);
			if (!m) continue;
			const changed = m.status !== event.status;
			if (changed) m.since = event.at;
			m.status = event.status;
			m.latency_ms = event.latency_ms ?? 0;
			m.checked_at = event.at;
			if (changed) this.load();
			return;
		}
	}
}

export const [getStatus, setStatus] = createContext<StatusStore>();
