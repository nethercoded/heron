import { api } from '#lib/api.ts';
import type { Page } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => ({
	page: await api.get<Page>('/admin/page', fetch)
});
