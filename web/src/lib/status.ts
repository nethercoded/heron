import type { Impact, IncidentStatus, Kind, Overall, Status } from './types.ts';

export const statusLabel: Record<Status, string> = {
	up: 'Operational',
	degraded: 'Degraded',
	down: 'Down',
	pending: 'Pending',
	paused: 'Paused',
	maintenance: 'Maintenance'
};

export const statusTone: Record<Status, string> = {
	up: 'bg-up',
	degraded: 'bg-degraded',
	down: 'bg-down',
	pending: 'bg-pending',
	paused: 'bg-paused',
	maintenance: 'bg-accent'
};

export const statusText: Record<Status, string> = {
	up: 'text-up',
	degraded: 'text-degraded',
	down: 'text-down',
	pending: 'text-muted',
	paused: 'text-muted',
	maintenance: 'text-accent'
};

export const statusPill: Record<Status, string> = {
	up: 'text-up bg-up/10 ring-up/25',
	degraded: 'text-degraded bg-degraded/12 ring-degraded/35',
	down: 'text-down bg-down/10 ring-down/30',
	pending: 'text-muted bg-sunken ring-line-strong',
	paused: 'text-muted bg-sunken ring-line-strong',
	maintenance: 'text-accent bg-accent/10 ring-accent/30'
};

export const overallCopy: Record<Overall, { title: string; detail: string; tone: Status }> = {
	operational: { title: 'All systems operational', detail: 'Every service is responding normally.', tone: 'up' },
	degraded: { title: 'Degraded performance', detail: 'Some services are slower than usual.', tone: 'degraded' },
	partial_outage: { title: 'Partial system outage', detail: 'Some services are unavailable.', tone: 'down' },
	major_outage: { title: 'Major system outage', detail: 'Most services are unavailable.', tone: 'down' },
	maintenance: { title: 'Scheduled maintenance', detail: 'Planned work is in progress.', tone: 'maintenance' }
};

export const impactLabel: Record<Impact, string> = {
	minor: 'Minor',
	major: 'Major',
	critical: 'Critical',
	maintenance: 'Maintenance'
};

export const impactTone: Record<Impact, string> = {
	minor: 'text-degraded bg-degraded/10 ring-degraded/30',
	major: 'text-down bg-down/10 ring-down/30',
	critical: 'text-down bg-down/15 ring-down/40',
	maintenance: 'text-accent bg-accent/10 ring-accent/30'
};

export const incidentStatusLabel: Record<IncidentStatus, string> = {
	investigating: 'Investigating',
	identified: 'Identified',
	monitoring: 'Monitoring',
	resolved: 'Resolved'
};

export const kindLabel: Record<Kind, string> = {
	http: 'HTTP',
	tcp: 'TCP',
	icmp: 'Ping',
	dns: 'DNS',
	push: 'Push'
};

export function dayTone(uptime: number | null, degraded = 0) {
	if (uptime == null) return 'bg-pending/60';
	if (uptime >= 0.999) return degraded > 0 ? 'bg-up/80' : 'bg-up';
	if (uptime >= 0.98) return 'bg-degraded';
	return 'bg-down';
}
