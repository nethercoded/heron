import { error } from '@sveltejs/kit';
import { ApiError, api } from '#lib/api.ts';
import type { AdminMonitor, MaintenanceWindow } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	try {
		const [window, monitors] = await Promise.all([
			api.get<MaintenanceWindow>(`/admin/maintenance/${params.id}`, fetch),
			api.get<AdminMonitor[]>('/admin/monitors', fetch)
		]);
		return { window, monitors };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'Maintenance window not found');
		throw err;
	}
};
