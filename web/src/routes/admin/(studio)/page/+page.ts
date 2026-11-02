import { api } from '#lib/api.ts';
import type { AdminMonitor, Page } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const [page, monitors] = await Promise.all([
		api.get<Page>('/admin/page', fetch),
		api.get<AdminMonitor[]>('/admin/monitors', fetch).catch(() => [] as AdminMonitor[])
	]);
	const groups = new Map<string, number>();
	for (const m of monitors) {
		if (m.public && m.group) groups.set(m.group, (groups.get(m.group) ?? 0) + 1);
	}
	return {
		page,
		groups: [...groups].map(([name, count]) => ({ name, count })).sort((a, b) => a.name.localeCompare(b.name))
	};
};
