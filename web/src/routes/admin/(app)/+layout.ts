import { redirect } from '@sveltejs/kit';
import { api } from '#lib/api.ts';
import type { AdminMonitor, Incident } from '#lib/types.ts';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent, fetch, depends }) => {
	const { auth } = await parent();
	if (auth.setup_required) redirect(307, '/admin/setup');
	if (!auth.user) redirect(307, '/admin/login');

	depends('heron:counts');
	const [monitors, incidents] = await Promise.all([
		api.get<AdminMonitor[]>('/admin/monitors', fetch).catch(() => []),
		api.get<Incident[]>('/admin/incidents', fetch).catch(() => [])
	]);

	return {
		user: auth.user,
		version: auth.version,
		counts: {
			monitors: monitors.length,
			down: monitors.filter((m) => m.status === 'down').length,
			incidents: incidents.filter((i) => !i.resolved_at).length
		}
	};
};
