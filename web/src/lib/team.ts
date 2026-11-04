import type { Role } from './types.ts';

export const roleLabel: Record<Role, string> = {
	owner: 'Owner',
	admin: 'Admin',
	viewer: 'Viewer'
};

export const roleHint: Record<Role, string> = {
	owner: 'Everything, including people, domain and security',
	admin: 'Monitors, incidents, maintenance, alerts and the status page',
	viewer: 'Sees the admin, cannot change anything'
};

export const canEdit = (role: Role | undefined) => role === 'owner' || role === 'admin';

const actions: Record<string, string> = {
	'auth.login': 'signed in',
	'auth.login_failed': 'failed to sign in',
	'auth.code_failed': 'entered a wrong 2FA code',
	'auth.logout': 'signed out',
	'user.setup': 'set up heron',
	'user.join': 'joined',
	'user.invite': 'created an invite',
	'user.invite_revoke': 'revoked an invite',
	'user.role': 'changed the role of',
	'user.delete': 'removed',
	'user.sign_out': 'signed out all sessions of',
	'user.2fa_reset': 'reset two-factor for',
	'account.password': 'changed their password',
	'account.2fa_on': 'turned on two-factor',
	'account.2fa_off': 'turned off two-factor',
	'account.passkey_add': 'added a passkey',
	'account.passkey_remove': 'removed a passkey',
	'monitor.create': 'created monitor',
	'monitor.update': 'edited monitor',
	'monitor.delete': 'deleted monitor',
	'monitor.pause': 'paused monitor',
	'monitor.resume': 'resumed monitor',
	'monitor.token': 'rotated the push token of',
	'monitor.reorder': 'reordered monitors',
	'monitor.import': 'imported from Uptime Kuma',
	'channel.create': 'added notification channel',
	'channel.update': 'edited notification channel',
	'channel.delete': 'deleted notification channel',
	'incident.create': 'opened incident',
	'incident.update': 'edited incident',
	'incident.post': 'posted an update to',
	'incident.delete': 'deleted incident',
	'maintenance.create': 'scheduled maintenance',
	'maintenance.update': 'edited maintenance',
	'maintenance.delete': 'deleted maintenance',
	'settings.domain': 'changed the domain to',
	'settings.page': 'edited the status page',
	'settings.metrics_token': 'issued a new metrics token',
	'settings.metrics_off': 'turned off metrics',
	'settings.auto_update': 'turned automatic updates',
	'settings.pelican': 'turned the Pelican integration',
	'update.install': 'installed heron',
	'backup.download': 'downloaded a backup'
};

export const actionLabel = (action: string) => actions[action] ?? action;

const linking: Record<string, string> = {
	'auth.login': 'with',
	'user.join': 'as',
	'user.invite': 'for a new'
};

export const targetLink = (action: string) => linking[action] ?? '';

export const auditGroups = [
	{ value: '', label: 'Everything' },
	{ value: 'auth', label: 'Sign ins' },
	{ value: 'monitor', label: 'Monitors' },
	{ value: 'incident', label: 'Incidents' },
	{ value: 'maintenance', label: 'Maintenance' },
	{ value: 'channel', label: 'Notifications' },
	{ value: 'user', label: 'People' },
	{ value: 'account', label: 'Account security' },
	{ value: 'settings', label: 'Settings' },
	{ value: 'update', label: 'Updates' },
	{ value: 'backup', label: 'Backups' }
];

export function actionTone(action: string) {
	if (action.endsWith('_failed') || action.endsWith('.delete') || action === 'user.delete') return 'var(--color-down)';
	if (action.startsWith('auth.')) return 'var(--color-faint)';
	if (action.endsWith('.create') || action === 'user.join') return 'var(--color-up)';
	return 'var(--color-accent)';
}
