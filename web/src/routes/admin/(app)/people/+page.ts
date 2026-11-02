import { api } from '#lib/api.ts';
import type { Invite, Member } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, depends }) => {
	depends('heron:people');
	return { team: await api.get<{ users: Member[]; invites: Invite[] }>('/admin/users', fetch) };
};
