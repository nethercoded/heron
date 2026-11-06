import { api } from '#lib/api.ts';
import type { AdminMonitor, Warning } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, depends }) => {
	depends('heron:monitors');
	const [monitors, warnings] = await Promise.all([
		api.get<AdminMonitor[]>('/admin/monitors', fetch),
		api.get<Warning[]>('/admin/warnings', fetch).catch(() => [] as Warning[])
	]);
	return { monitors, warnings };
};
