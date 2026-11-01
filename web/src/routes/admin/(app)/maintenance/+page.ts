import { api } from '#lib/api.ts';
import type { MaintenanceWindow } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, depends }) => {
	depends('heron:maintenance');
	return { windows: await api.get<MaintenanceWindow[]>('/admin/maintenance', fetch) };
};
