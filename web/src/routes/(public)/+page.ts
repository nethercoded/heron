import { api } from '#lib/api.ts';
import type { Incident } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	try {
		return { recent: await api.get<Incident[]>('/incidents?days=7', fetch) };
	} catch {
		return { recent: [] as Incident[] };
	}
};
