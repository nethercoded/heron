import { api } from '#lib/api.ts';
import type { StatusPage } from '#lib/types.ts';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ fetch }) => {
	try {
		return { status: await api.get<StatusPage>('/status', fetch) };
	} catch {
		return { status: null };
	}
};
