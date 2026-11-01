import { error } from '@sveltejs/kit';
import { ApiError, api } from '#lib/api.ts';
import type { Calendar, MonitorDetail } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	const slug = encodeURIComponent(params.slug);
	try {
		const [detail, calendar] = await Promise.all([
			api.get<MonitorDetail>(`/monitors/${slug}`, fetch),
			api.get<Calendar>(`/monitors/${slug}/calendar`, fetch)
		]);
		return { detail, calendar };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'This service does not exist');
		throw err;
	}
};
