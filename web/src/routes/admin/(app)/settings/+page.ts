import { api } from '#lib/api.ts';
import type { Page, SiteSettings } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const [page, site] = await Promise.all([
		api.get<Page>('/admin/page', fetch),
		api.get<SiteSettings>('/admin/site', fetch)
	]);
	return { page, site };
};
