import { api } from '#lib/api.ts';
import type { Channel } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => ({
	channels: await api.get<Channel[]>('/admin/channels', fetch)
});
