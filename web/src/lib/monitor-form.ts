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

export const kindGroups: { label: string; kinds: { value: Kind; label: string; hint: string }[] }[] = [
	{
		label: 'Network',
		kinds: [
			{ value: 'http', label: 'HTTP', hint: 'Websites and APIs' },
			{ value: 'tcp', label: 'TCP', hint: 'Any open port' },
			{ value: 'icmp', label: 'Ping', hint: 'Host reachability' },
			{ value: 'dns', label: 'DNS', hint: 'Record resolution' },
			{ value: 'grpc', label: 'gRPC', hint: 'Health protocol' }
		]
	},
	{
		label: 'Services',
		kinds: [
			{ value: 'postgres', label: 'Postgres', hint: 'Runs a query' },
			{ value: 'redis', label: 'Redis', hint: 'AUTH and PING' },
			{ value: 'smtp', label: 'SMTP', hint: 'Mail delivery' },
			{ value: 'imap', label: 'IMAP', hint: 'Mailbox access' },
			{ value: 'docker', label: 'Docker', hint: 'Container health' }
		]
	},
	{
		label: 'Games and jobs',
		kinds: [
			{ value: 'minecraft', label: 'Minecraft', hint: 'Java server ping' },
			{ value: 'steam', label: 'Steam', hint: 'Source query' },
			{ value: 'push', label: 'Push', hint: 'Cron jobs, backups' }
		]
	}
];

export const targetCopy: Record<Kind, { label: string; placeholder: string; hint?: string }> = {
	http: { label: 'URL', placeholder: 'https://example.com/health' },
	tcp: { label: 'Host and port', placeholder: 'db.internal:5432' },
	icmp: { label: 'Host', placeholder: '203.0.113.10' },
	dns: { label: 'Domain', placeholder: 'example.com' },
	push: { label: '', placeholder: '' },
	grpc: { label: 'Host and port', placeholder: 'api.internal:50051' },
	postgres: {
		label: 'Connection URL',
		placeholder: 'postgres://heron:secret@db.internal:5432/app?sslmode=disable',
		hint: 'Use a read-only role. The password is never shown in alerts.'
	},
	redis: { label: 'Host and port', placeholder: 'cache.internal:6379', hint: 'Port defaults to 6379' },
	smtp: { label: 'Host and port', placeholder: 'mail.example.com:587', hint: 'Port defaults to 25, or 465 with TLS' },
	imap: { label: 'Host and port', placeholder: 'mail.example.com', hint: 'Port defaults to 143, or 993 with TLS' },
	docker: { label: 'Container', placeholder: 'web-1', hint: 'Name or id of the container' },
	minecraft: { label: 'Server address', placeholder: 'play.example.com', hint: 'Port defaults to 25565' },
	steam: { label: 'Server address', placeholder: '203.0.113.10:27015', hint: 'Query port, defaults to 27015' }
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

export const commonKinds: { value: Kind; label: string }[] = [
	{ value: 'http', label: 'Website or API' },
	{ value: 'tcp', label: 'Open port' },
	{ value: 'icmp', label: 'Ping' },
	{ value: 'push', label: 'Cron job heartbeat' }
];

export const otherKinds: { value: Kind; label: string }[] = [
	{ value: 'dns', label: 'DNS record' },
	{ value: 'postgres', label: 'Postgres' },
	{ value: 'redis', label: 'Redis' },
	{ value: 'smtp', label: 'Mail server (SMTP)' },
	{ value: 'imap', label: 'Mailbox (IMAP)' },
	{ value: 'docker', label: 'Docker container' },
	{ value: 'grpc', label: 'gRPC service' },
	{ value: 'minecraft', label: 'Minecraft server' },
	{ value: 'steam', label: 'Steam game server' }
];

export const kindLabel = (kind: Kind) => [...commonKinds, ...otherKinds].find((k) => k.value === kind)?.label ?? kind;

const schemes: Record<string, Kind> = {
	http: 'http',
	https: 'http',
	postgres: 'postgres',
	postgresql: 'postgres',
	redis: 'redis',
	rediss: 'redis',
	grpc: 'grpc'
};

const ports: Record<string, Kind> = {
	'80': 'http',
	'443': 'http',
	'3000': 'http',
	'5000': 'http',
	'8000': 'http',
	'8080': 'http',
	'8443': 'http',
	'6379': 'redis',
	'25': 'smtp',
	'465': 'smtp',
	'587': 'smtp',
	'143': 'imap',
	'993': 'imap',
	'25565': 'minecraft',
	'27015': 'steam',
	'50051': 'grpc'
};

export function detect(raw: string): Kind | null {
	const value = raw.trim();
	if (!value) return null;
	const scheme = value.match(/^([a-z]+):\/\//i)?.[1]?.toLowerCase();
	if (scheme) return schemes[scheme] ?? 'http';
	const port = value.match(/^[^/\s]+:(\d+)$/)?.[1];
	if (port) return ports[port] ?? 'tcp';
	if (/^localhost(\/.*)?$/i.test(value)) return 'http';
	if (/^[\d.]+$/.test(value) || /^[0-9a-f]*:[0-9a-f:]+$/i.test(value)) return 'icmp';
	if (/^[^\s/]+\.[^\s/.]+(\/.*)?$/.test(value)) return 'http';
	return null;
}

export function normalizeTarget(kind: Kind, raw: string) {
	const value = raw.trim();
	if (kind === 'http' && value && !/^https?:\/\//i.test(value)) {
		const port = value.match(/^[^/]+:(\d+)/)?.[1];
		const local = /^(localhost|127\.|10\.|192\.168\.)/.test(value);
		return `${(port && port !== '443') || local ? 'http' : 'https'}://${value}`;
	}
	if (kind === 'grpc') return value.replace(/^grpc:\/\//i, '');
	if (kind === 'redis' && /^rediss?:\/\//i.test(value)) {
		try {
			return new URL(value).host;
		} catch {
			return value;
		}
	}
	return value;
}

export function nameFrom(kind: Kind, raw: string) {
	const value = raw.trim();
	if (!value) return '';
	try {
		if (/^[a-z]+:\/\//i.test(value)) return new URL(value).hostname.replace(/^www\./, '');
		if (kind === 'http') return new URL(`https://${value}`).hostname.replace(/^www\./, '');
	} catch {}
	return value.replace(/:\d+$/, '');
}
