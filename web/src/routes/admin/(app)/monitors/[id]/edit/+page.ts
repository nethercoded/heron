import { api } from '#lib/api.ts';
import type { AdminMonitor, Channel } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	const [detail, channels, monitors] = await Promise.all([
		api.get<{ monitor: AdminMonitor }>(`/admin/monitors/${params.id}`, fetch),
		api.get<Channel[]>('/admin/channels', fetch),
		api.get<AdminMonitor[]>('/admin/monitors', fetch)
	]);
	return { monitor: detail.monitor, channels, monitors };
};
