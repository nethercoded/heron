import type { AdminMonitor, Kind, MonitorInput } from './types.ts';

export const intervals = [
	{ value: 20, label: '20 seconds' },
	{ value: 30, label: '30 seconds' },
	{ value: 60, label: '1 minute' },
	{ value: 120, label: '2 minutes' },
	{ value: 300, label: '5 minutes' },
	{ value: 600, label: '10 minutes' },
	{ value: 1800, label: '30 minutes' },
	{ value: 3600, label: '1 hour' },
	{ value: 21600, label: '6 hours' },
	{ value: 86400, label: '1 day' }
];

export const kinds: { value: Kind; label: string; hint: string }[] = [
	{ value: 'http', label: 'HTTP', hint: 'Websites and APIs' },
	{ value: 'tcp', label: 'TCP', hint: 'Any open port' },
	{ value: 'icmp', label: 'Ping', hint: 'Host reachability' },
	{ value: 'dns', label: 'DNS', hint: 'Record resolution' },
	{ value: 'push', label: 'Push', hint: 'Cron jobs, backups' }
];

export const targetCopy: Record<Kind, { label: string; placeholder: string }> = {
	http: { label: 'URL', placeholder: 'https://example.com/health' },
	tcp: { label: 'Host and port', placeholder: 'db.internal:5432' },
	icmp: { label: 'Host', placeholder: '203.0.113.10' },
	dns: { label: 'Domain', placeholder: 'example.com' },
	push: { label: '', placeholder: '' }
};

export function blank(): MonitorInput {
	return {
		name: '',
		slug: '',
		description: '',
		group: '',
		kind: 'http',
		target: '',
		interval_s: 60,
		timeout_s: 10,
		retries: 1,
		public: true,
		paused: false,
		settings: {},
		channel_ids: []
	};
}

export function fromMonitor(m: AdminMonitor): MonitorInput {
	return {
		name: m.name,
		slug: m.slug,
		description: m.description,
		group: m.group,
		kind: m.kind,
		target: m.target,
		interval_s: m.interval_s,
		timeout_s: m.timeout_s,
		retries: m.retries,
		public: m.public,
		paused: m.paused,
		settings: JSON.parse(JSON.stringify(m.settings)),
		channel_ids: [...m.channel_ids]
	};
}

export function slugify(value: string) {
	return value
		.toLowerCase()
		.normalize('NFKD')
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '');
}

export function parseHeaders(text: string) {
	const headers: Record<string, string> = {};
	for (const line of text.split('\n')) {
		const at = line.indexOf(':');
		if (at <= 0) continue;
		const key = line.slice(0, at).trim();
		if (key) headers[key] = line.slice(at + 1).trim();
	}
	return headers;
}

export function formatHeaders(headers: Record<string, string> | undefined) {
	return Object.entries(headers ?? {})
		.map(([k, v]) => `${k}: ${v}`)
		.join('\n');
}

export function parseCodes(text: string) {
	return text
		.split(/[\s,]+/)
		.map(Number)
		.filter((n) => Number.isInteger(n) && n > 0);
}
