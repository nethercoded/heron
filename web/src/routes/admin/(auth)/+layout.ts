import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent }) => {
	const { auth } = await parent();
	if (auth.user) redirect(307, '/admin');
	if (auth.setup_required) redirect(307, '/admin/install');
};
