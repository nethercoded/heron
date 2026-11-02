import { api } from '#lib/api.ts';
import type { AuditEntry, Member } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const query = new URLSearchParams();
	for (const key of ['action', 'user']) {
		const value = url.searchParams.get(key);
		if (value) query.set(key, value);
	}
	const [entries, team] = await Promise.all([
		api.get<AuditEntry[]>(`/admin/audit?${query}`, fetch),
		api.get<{ users: Member[] }>('/admin/users', fetch)
	]);
	return {
		entries,
		users: team.users,
		action: url.searchParams.get('action') ?? '',
		userId: url.searchParams.get('user') ?? ''
	};
};
