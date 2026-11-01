const relative = new Intl.RelativeTimeFormat('en', { numeric: 'auto', style: 'long' });
const dateTime = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' });
const dateOnly = new Intl.DateTimeFormat('en', { dateStyle: 'medium' });
const monthYear = new Intl.DateTimeFormat('en', { month: 'long', year: 'numeric' });
const timeOnly = new Intl.DateTimeFormat('en', { timeStyle: 'short' });

const steps: [Intl.RelativeTimeFormatUnit, number][] = [
	['year', 31_536_000],
	['month', 2_592_000],
	['week', 604_800],
	['day', 86_400],
	['hour', 3_600],
	['minute', 60],
	['second', 1]
];

export function ago(iso: string | undefined, now = Date.now()) {
	if (!iso) return 'never';
	const seconds = Math.round((new Date(iso).getTime() - now) / 1000);
	if (Math.abs(seconds) < 5) return 'just now';
	for (const [unit, size] of steps) {
		if (Math.abs(seconds) >= size || unit === 'second') {
			return relative.format(Math.round(seconds / size), unit);
		}
	}
	return '';
}

export function duration(ms: number) {
	const s = Math.max(0, Math.round(ms / 1000));
	const d = Math.floor(s / 86_400);
	const h = Math.floor((s % 86_400) / 3_600);
	const m = Math.floor((s % 3_600) / 60);
	if (d > 0) return h > 0 ? `${d}d ${h}h` : `${d}d`;
	if (h > 0) return m > 0 ? `${h}h ${m}m` : `${h}h`;
	if (m > 0) return `${m}m`;
	return `${s}s`;
}

export function between(from: string, to?: string) {
	return duration((to ? new Date(to) : new Date()).getTime() - new Date(from).getTime());
}

export function percent(ratio: number | null | undefined, digits = 2) {
	if (ratio == null) return '—';
	const value = ratio * 100;
	if (value === 100) return '100%';
	return `${Math.min(value, 99.99).toFixed(digits)}%`;
}

export function latency(ms: number | undefined) {
	if (!ms) return '—';
	return ms >= 1000 ? `${(ms / 1000).toFixed(2)} s` : `${Math.round(ms)} ms`;
}

export function interval(seconds: number) {
	if (seconds % 86_400 === 0) return `${seconds / 86_400}d`;
	if (seconds % 3_600 === 0) return `${seconds / 3_600}h`;
	if (seconds % 60 === 0) return `${seconds / 60}m`;
	return `${seconds}s`;
}

export const formatDateTime = (iso: string) => dateTime.format(new Date(iso));
export const formatDate = (iso: string) => dateOnly.format(new Date(iso));
export const formatMonth = (iso: string) => monthYear.format(new Date(iso));
export const formatTime = (iso: string) => timeOnly.format(new Date(iso));

export function formatDay(date: string) {
	const [y, m, d] = date.split('-').map(Number);
	return dateOnly.format(new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1));
}
