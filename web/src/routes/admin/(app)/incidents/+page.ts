import { api } from '#lib/api.ts';
import type { Incident } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => ({
	incidents: await api.get<Incident[]>('/admin/incidents', fetch)
});
