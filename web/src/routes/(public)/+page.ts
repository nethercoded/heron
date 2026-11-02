import { api } from '#lib/api.ts';
import { historySpan } from '#lib/blocks.ts';
import type { Incident } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, parent }) => {
	const { status } = await parent();
	const days = historySpan(status?.page.blocks ?? []);
	try {
		return { recent: await api.get<Incident[]>(`/incidents?days=${days}`, fetch) };
	} catch {
		return { recent: [] as Incident[] };
	}
};
