import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent, url }) => {
	const { auth } = await parent();
	const step = Number(url.searchParams.get('step') ?? 0);
	if (auth.setup_required) return { step: Math.min(step, 2) };
	if (!auth.user) redirect(307, '/admin/login');
	if (step < 3) redirect(307, '/admin');
	return { step };
};
