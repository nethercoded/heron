import { api } from '#lib/api.ts';
import type { AuthState } from '#lib/types.ts';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ fetch, depends }) => {
	depends('heron:auth');
	return { auth: await api.get<AuthState>('/auth/state', fetch) };
};
