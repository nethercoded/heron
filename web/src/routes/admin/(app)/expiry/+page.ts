import { api } from '#lib/api.ts';
import type { ExpiryList } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, depends }) => {
	depends('heron:expiry');
	return { expiry: await api.get<ExpiryList>('/admin/expiry', fetch) };
};
