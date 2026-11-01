import { error } from '@sveltejs/kit';
import { ApiError, api } from '#lib/api.ts';
import type { MonitorDetail } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	try {
		return { detail: await api.get<MonitorDetail>(`/monitors/${encodeURIComponent(params.slug)}`, fetch) };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'This service does not exist');
		throw err;
	}
};
