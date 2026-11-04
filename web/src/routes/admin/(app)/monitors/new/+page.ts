import { api } from '#lib/api.ts';
import type { AdminMonitor, Channel } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const [channels, monitors] = await Promise.all([
		api.get<Channel[]>('/admin/channels', fetch),
		api.get<AdminMonitor[]>('/admin/monitors', fetch)
	]);
	return { channels, monitors };
};
