import { redirect } from '@sveltejs/kit';
import { api } from '#lib/api.ts';
import type { AdminMonitor, Incident, Updates } from '#lib/types.ts';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent, fetch, depends }) => {
	const { auth } = await parent();
	if (auth.setup_required) redirect(307, '/admin/install');
	if (!auth.user) redirect(307, '/admin/login');

	depends('heron:counts', 'heron:updates');
	const [monitors, incidents, updates] = await Promise.all([
		api.get<AdminMonitor[]>('/admin/monitors', fetch).catch(() => []),
		api.get<Incident[]>('/admin/incidents', fetch).catch(() => []),
		auth.user.role === 'owner' ? api.get<Updates>('/admin/updates', fetch).catch(() => null) : null
	]);

	return {
		user: auth.user,
		version: auth.version,
		updates,
		counts: {
			monitors: monitors.length,
			down: monitors.filter((m) => m.status === 'down').length,
			incidents: incidents.filter((i) => !i.resolved_at).length
		}
	};
};
