<script lang="ts">
	import { untrack } from 'svelte';
	import { api, message } from '#lib/api.ts';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { formatDate, formatDateTime, formatTime } from '#lib/format.ts';
	import { actionLabel, actionTone, auditGroups, targetLink } from '#lib/team.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { AuditEntry } from '#lib/types.ts';
	import { goto } from '$app/navigation';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let entries = $state<AuditEntry[]>(untrack(() => data.entries));
	let more = $state(untrack(() => data.entries.length === 50));
	let loading = $state(false);

	$effect.pre(() => {
		entries = data.entries;
		more = data.entries.length === 50;
	});

	const days = $derived.by(() => {
		const out: { day: string; items: AuditEntry[] }[] = [];
		for (const e of entries) {
			const day = formatDate(e.at);
			const last = out.at(-1);
			if (last?.day === day) last.items.push(e);
			else out.push({ day, items: [e] });
		}
		return out;
	});

	function filter(key: 'action' | 'user', value: string) {
		const query = new URLSearchParams({ action: data.action, user: data.userId, [key]: value });
		for (const [k, v] of [...query]) if (!v) query.delete(k);
		goto(query.size ? `?${query}` : '/admin/audit');
	}

	async function loadMore() {
		const last = entries.at(-1);
		if (!last) return;
		loading = true;
		try {
			const query = new URLSearchParams({ before: String(last.id) });
			if (data.action) query.set('action', data.action);
			if (data.userId) query.set('user', data.userId);
			const page = await api.get<AuditEntry[]>(`/admin/audit?${query}`);
			entries = [...entries, ...page];
			more = page.length === 50;
		} catch (err) {
			toast.error(message(err));
		} finally {
			loading = false;
		}
	}
</script>

<PageHeader title="Audit log" description="Who changed what, and when. Kept for a year." />

<section class="card overflow-hidden">
	<header class="flex flex-wrap items-center gap-2 px-[22px] pt-[18px] pb-4">
		<h2 class="mr-auto text-[22px] leading-tight font-semibold tracking-[-0.015em]">Activity</h2>
		<select class="input-select h-10 w-full sm:w-48" value={data.action} onchange={(e) => filter('action', e.currentTarget.value)} aria-label="Kind of activity">
			{#each auditGroups as g (g.value)}<option value={g.value}>{g.label}</option>{/each}
		</select>
		<select class="input-select h-10 w-full sm:w-44" value={data.userId} onchange={(e) => filter('user', e.currentTarget.value)} aria-label="Person">
			<option value="">Everyone</option>
			{#each data.users as u (u.id)}<option value={String(u.id)}>{u.username}</option>{/each}
		</select>
	</header>

	{#each days as group (group.day)}
		<h3 class="border-t border-line bg-surface px-[22px] py-2 font-mono text-[11px] font-semibold tracking-wider text-faint uppercase">{group.day}</h3>
		<ul class="divide-y divide-line border-t border-line">
			{#each group.items as e (e.id)}
				<li class="grid grid-cols-[52px_minmax(0,1fr)] items-baseline gap-x-4 px-[22px] py-3 sm:grid-cols-[64px_minmax(0,1fr)_auto]">
					<time class="font-mono text-xs text-faint" datetime={e.at} title={formatDateTime(e.at)}>{formatTime(e.at)}</time>
					<p class="min-w-0 text-[14px] text-ink-2">
						<span class="sq mr-2 align-middle" style="--c: {actionTone(e.action)}; --s: 7px"></span>
						<span class="font-semibold text-ink">{e.username || 'someone'}</span>
						{actionLabel(e.action)}
						{#if e.target}{targetLink(e.action)} <span class="font-medium break-words text-ink">{e.target}</span>{/if}
					</p>
					<span class="col-start-2 font-mono text-[11px] text-faint sm:col-start-3">{e.ip}</span>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="border-t border-line px-6 py-14 text-center text-ink-2">Nothing recorded yet.</p>
	{/each}

	{#if more}
		<div class="border-t border-line p-4 text-center">
			<button class="btn-secondary btn-sm" disabled={loading} onclick={loadMore}>{loading ? 'Loading…' : 'Show older'}</button>
		</div>
	{/if}
</section>
