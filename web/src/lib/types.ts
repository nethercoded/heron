export type Status = 'pending' | 'up' | 'degraded' | 'down' | 'paused';
export type Kind = 'http' | 'tcp' | 'icmp' | 'dns' | 'push';
export type Overall = 'operational' | 'degraded' | 'partial_outage' | 'major_outage' | 'maintenance';
export type Impact = 'minor' | 'major' | 'critical' | 'maintenance';
export type IncidentStatus = 'investigating' | 'identified' | 'monitoring' | 'resolved';
export type ChannelKind = 'telegram' | 'webhook';
export type Range = '24h' | '7d' | '30d' | '90d';

export interface Page {
	title: string;
	description?: string;
	logo_url?: string;
	home_url?: string;
	footer?: string;
	history_days?: number;
}

export interface Day {
	date: string;
	uptime: number | null;
	checks: number;
	down: number;
	degraded: number;
}

export interface Uptime {
	day: number;
	week: number;
	month: number;
	history: number;
}

export interface PublicMonitor {
	slug: string;
	name: string;
	description?: string;
	group: string;
	kind: Kind;
	status: Status;
	since?: string;
	checked_at?: string;
	latency_ms: number;
	uptime: Uptime;
	days: Day[];
}

export interface Group {
	name: string;
	monitors: PublicMonitor[];
}

export interface IncidentUpdate {
	id: number;
	status: IncidentStatus;
	body: string;
	created_at: string;
}

export interface Incident {
	id: number;
	title: string;
	impact: Impact;
	status: IncidentStatus;
	automatic: boolean;
	started_at: string;
	resolved_at?: string;
	updated_at: string;
	monitors: { slug: string; name: string }[];
	monitor_ids?: number[];
	updates: IncidentUpdate[];
}

export interface StatusPage {
	page: Page;
	status: Overall;
	groups: Group[];
	incidents: Incident[];
	updated_at: string;
}

export interface Beat {
	at: string;
	status: Status;
	latency_ms: number;
	message?: string;
}

export interface MonitorDetail {
	monitor: PublicMonitor;
	beats: Beat[];
	incidents: Incident[];
}

export interface Point {
	at: string;
	avg_ms: number;
	max_ms: number;
	checks: number;
	down: number;
}

export interface Series {
	step_s: number;
	points: Point[];
}

export interface Settings {
	method?: string;
	headers?: Record<string, string>;
	body?: string;
	accepted_status?: number[];
	keyword?: string;
	keyword_absent?: boolean;
	max_redirects?: number;
	ignore_tls?: boolean;
	cert_expiry_days?: number;
	degraded_after_ms?: number;
	record_type?: string;
	resolver?: string;
	expect?: string;
	packets?: number;
	grace_seconds?: number;
}

export interface MonitorInput {
	name: string;
	slug: string;
	description: string;
	group: string;
	kind: Kind;
	target: string;
	interval_s: number;
	timeout_s: number;
	retries: number;
	public: boolean;
	paused: boolean;
	settings: Settings;
	channel_ids: number[];
}

export interface AdminMonitor extends MonitorInput {
	id: number;
	position: number;
	push_token?: string;
	status: Status;
	since?: string;
	checked_at?: string;
	latency_ms: number;
	message: string;
	uptime_24h?: number;
	created_at: string;
	updated_at: string;
}

export interface ChannelConfig {
	bot_token?: string;
	chat_id?: string;
	thread_id?: number;
	url?: string;
	secret?: string;
	headers?: Record<string, string>;
}

export interface Channel {
	id: number;
	name: string;
	kind: ChannelKind;
	enabled: boolean;
	config: ChannelConfig;
	created_at: string;
	updated_at: string;
}

export interface AuthState {
	setup_required: boolean;
	user: { username: string } | null;
	version: string;
}

export interface LiveMonitor {
	slug: string;
	status: Status;
	latency_ms?: number;
	at?: string;
	id?: number;
	message?: string;
}
