import { error } from '@sveltejs/kit';
import { ApiError, api } from '#lib/api.ts';
import type { Incident } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	try {
		return { incident: await api.get<Incident>(`/incidents/${params.id}`, fetch) };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'This incident does not exist');
		throw err;
	}
};
