import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent }) => {
	const { auth } = await parent();
	if (auth.setup_required) redirect(307, '/admin/setup');
	if (!auth.user) redirect(307, '/admin/login');
	return { user: auth.user, version: auth.version };
};
