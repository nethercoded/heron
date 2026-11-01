import { error } from '@sveltejs/kit';
import { ApiError, api } from '#lib/api.ts';
import type { AdminMonitor, Beat } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, depends }) => {
	depends('heron:monitor');
	try {
		return await api.get<{ monitor: AdminMonitor; beats: Beat[] }>(`/admin/monitors/${params.id}`, fetch);
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'Monitor not found');
		throw err;
	}
};
