import { error } from '@sveltejs/kit';
import { ApiError, api } from '#lib/api.ts';
import type { AdminMonitor, Beat, Calendar, Warning } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, depends }) => {
	depends('heron:monitor');
	try {
		const [detail, calendar, warnings] = await Promise.all([
			api.get<{ monitor: AdminMonitor; beats: Beat[] }>(`/admin/monitors/${params.id}`, fetch),
			api.get<Calendar>(`/admin/monitors/${params.id}/calendar`, fetch),
			api.get<Warning[]>('/admin/warnings', fetch).catch(() => [] as Warning[])
		]);
		return { ...detail, calendar, warnings: warnings.filter((w) => w.monitor_id === detail.monitor.id) };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'Monitor not found');
		throw err;
	}
};
