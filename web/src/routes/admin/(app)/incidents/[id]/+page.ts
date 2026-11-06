import { error } from '@sveltejs/kit';
import { ApiError, api } from '#lib/api.ts';
import type { AdminMonitor, Evidence, Incident } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, depends }) => {
	depends('heron:incident');
	try {
		const [incident, monitors, evidence] = await Promise.all([
			api.get<Incident>(`/admin/incidents/${params.id}`, fetch),
			api.get<AdminMonitor[]>('/admin/monitors', fetch),
			api.get<Evidence[]>(`/admin/incidents/${params.id}/evidence`, fetch).catch(() => [] as Evidence[])
		]);
		return { incident, monitors, evidence };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'Incident not found');
		throw err;
	}
};
