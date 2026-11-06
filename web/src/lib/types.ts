export type Status = 'pending' | 'up' | 'degraded' | 'down' | 'paused' | 'maintenance';
export type Repeat = 'none' | 'daily' | 'weekly';

export interface MaintenanceInput {
	title: string;
	body: string;
	monitor_ids: number[];
	starts_at: string;
	duration_s: number;
	repeat: Repeat;
	enabled: boolean;
}

export interface MaintenanceWindow {
	id: number;
	title: string;
	body: string;
	starts_at: string;
	ends_at: string;
	duration_s: number;
	repeat: Repeat;
	active: boolean;
	enabled?: boolean;
	finished?: boolean;
	monitors: { slug: string; name: string }[];
	monitor_ids?: number[];
}
export type Kind =
	| 'http'
	| 'tcp'
	| 'icmp'
	| 'dns'
	| 'push'
	| 'redis'
	| 'postgres'
	| 'smtp'
	| 'imap'
	| 'docker'
	| 'grpc'
	| 'minecraft'
	| 'steam'
	| 'pelican_node'
	| 'pelican_server';
export type Overall = 'operational' | 'degraded' | 'partial_outage' | 'major_outage' | 'maintenance';
export type Impact = 'minor' | 'major' | 'critical' | 'maintenance';
export type IncidentStatus = 'investigating' | 'identified' | 'monitoring' | 'resolved';
export type ChannelKind = 'telegram' | 'webhook' | 'discord';
export type Range = '24h' | '7d' | '30d' | '90d';

export interface Page {
	title: string;
	description?: string;
	logo_url?: string;
	home_url?: string;
	footer?: string;
	history_days?: number;
	blocks: Block[];
}

export type BlockType = 'hero' | 'notice' | 'incidents' | 'maintenance' | 'monitors' | 'history';

export type Tone = 'info' | 'success' | 'warning' | 'danger';

export interface Block {
	id: string;
	type: BlockType;
	hidden?: boolean;
	title?: string;
	body?: string;
	tone?: Tone;
	group?: string;
	style?: string;
	days?: number;
	stats?: string;
	width?: number;
}

export interface Day {
	date: string;
	uptime: number | null;
	checks: number;
	down: number;
	degraded: number;
}

export interface Calendar {
	since: string;
	days: Day[];
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
	maintenance: MaintenanceWindow[];
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
	password?: string;
	tls?: boolean;
	starttls?: boolean;
	service?: string;
	docker_host?: string;
	min_players?: number;
	depends_on?: number;
	recover_after?: number;
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
	mention?: string;
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

export interface SiteSettings {
	domain: string;
	mode: '' | 'direct' | 'proxy';
	email: string;
	url: string;
	reachable: boolean;
}

export type Role = 'owner' | 'admin' | 'viewer';

export interface User {
	id: number;
	username: string;
	role: Role;
	two_factor: boolean;
	created_at: string;
	last_seen_at?: string;
}

export interface Member extends User {
	sessions: number;
	passkeys: number;
}

export interface Invite {
	id: number;
	role: Role;
	created_by: string;
	created_at: string;
	expires_at: string;
	token?: string;
}

export interface Passkey {
	id: string;
	name: string;
	created_at: string;
	last_used_at?: string;
}

export interface AuditEntry {
	id: number;
	at: string;
	user_id?: number;
	username: string;
	action: string;
	target: string;
	ip: string;
}

export interface LoginResult {
	user?: User;
	ticket?: string;
}

export interface AuthState {
	setup_required: boolean;
	user: User | null;
	version: string;
	demo?: { username: string; password: string };
}

export interface LiveMonitor {
	slug: string;
	status: Status;
	latency_ms?: number;
	at?: string;
	id?: number;
	message?: string;
}

export interface Release {
	version: string;
	url: string;
	notes: string;
	published_at: string;
}

export interface Updates {
	current: string;
	latest: Release | null;
	available: boolean;
	checked_at?: string;
	error?: string;
	blocked?: string;
	busy: boolean;
	failed?: string;
	auto: boolean;
}

export interface ExpiryItem {
	host: string;
	kind: 'domain' | 'cert';
	expires_at?: string;
	days_left: number | null;
	checked_at?: string;
	issuer?: string;
	error?: string;
	monitors: { id: number; name: string }[];
}

export interface ExpiryList {
	items: ExpiryItem[];
	checking: boolean;
}

export interface PelicanSettings {
	enabled: boolean;
	url: string;
	has_key: boolean;
	public: boolean;
	channel_ids: number[];
	status: { synced_at?: string; nodes: number; servers: number; error?: string; syncing: boolean };
}

export interface BlackBoxReport {
	at: string;
	target: string;
	reason: string;
	dns?: { host: string; addresses?: string[]; cname?: string; took_ms: number; error?: string };
	connect?: { address: string; took_ms: number; error?: string };
	tls?: {
		version?: string;
		valid: boolean;
		error?: string;
		chain?: { subject: string; issuer: string; not_after: string; names?: string[] }[];
	};
	response?: {
		status?: string;
		code?: number;
		remote?: string;
		headers?: { name: string; value: string }[];
		body?: string;
		cut?: boolean;
		redirect?: string;
		error?: string;
		timing: { dns_ms: number; connect_ms: number; tls_ms: number; first_byte_ms: number; total_ms: number };
	};
	route?: { tool: string; output?: string; error?: string };
	notes?: string[];
}

export interface Evidence {
	id: number;
	monitor_id: number;
	monitor_name: string;
	at: string;
	report: BlackBoxReport;
	image: boolean;
}

export interface Warning {
	monitor_id: number;
	monitor_name: string;
	kind: 'slow' | 'hidden_failures' | 'memory' | 'disk';
	message: string;
	since: string;
}
