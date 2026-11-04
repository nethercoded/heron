import { api } from '#lib/api.ts';
import type { Channel, Page, PelicanSettings, SiteSettings } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const [page, site, pelican, channels] = await Promise.all([
		api.get<Page>('/admin/page', fetch),
		api.get<SiteSettings>('/admin/site', fetch),
		api.get<PelicanSettings>('/admin/pelican', fetch).catch(() => null),
		api.get<Channel[]>('/admin/channels', fetch).catch(() => [] as Channel[])
	]);
	return { page, site, pelican, channels };
};
