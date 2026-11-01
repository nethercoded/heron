import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent, url }) => {
	const { auth } = await parent();
	if (auth.user) redirect(307, '/admin');
	if (auth.setup_required && url.pathname !== '/admin/setup') redirect(307, '/admin/setup');
	if (!auth.setup_required && url.pathname === '/admin/setup') redirect(307, '/admin/login');
};
