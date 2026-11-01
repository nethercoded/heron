import { api } from '#lib/api.ts';
import type { AdminMonitor } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, depends }) => {
	depends('heron:monitors');
	return { monitors: await api.get<AdminMonitor[]>('/admin/monitors', fetch) };
};
