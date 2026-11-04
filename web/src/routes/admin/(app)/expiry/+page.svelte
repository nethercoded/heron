<script lang="ts">
	import { api, message } from '#lib/api.ts';
	import Icon from '#lib/components/Icon.svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import { ago } from '#lib/format.ts';
	import { canEdit } from '#lib/team.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { ExpiryItem } from '#lib/types.ts';
	import { invalidate } from '$app/navigation';
	import { page } from '$app/state';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const editor = $derived(canEdit(page.data.user?.role));
	const checking = $derived(data.expiry.checking);
	const soon = $derived(data.expiry.items.filter((i) => i.days_left != null && i.days_left <= 30).length);

	$effect(() => {
		if (!checking) return;
		const id = setInterval(() => invalidate('heron:expiry'), 2000);
		return () => clearInterval(id);
	});

	async function check() {
		try {
			await api.post('/admin/expiry/check');
			await invalidate('heron:expiry');
		} catch (err) {
			toast.error(message(err));
		}
	}

	function tone(item: ExpiryItem) {
		if (item.error) return 'var(--color-faint)';
		if (item.days_left == null) return 'var(--color-faint)';
		if (item.days_left <= 7) return 'var(--color-down)';
		if (item.days_left <= 30) return 'var(--color-degraded)';
		return 'var(--color-up)';
	}

	function left(item: ExpiryItem) {
		if (item.days_left == null) return item.error ? 'Unknown' : 'Not checked yet';
		if (item.days_left <= 0) return 'Expired';
		if (item.days_left === 1) return 'Tomorrow';
		return `${item.days_left} days`;
	}

	const date = (iso?: string) =>
		iso ? new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' }) : '';
	const host = (item: ExpiryItem) => (item.kind === 'cert' ? item.host.replace(/:443$/, '') : item.host);
</script>

<PageHeader
	title="Expiry"
	description="Domains and certificates behind your monitors. heron checks them twice a day and warns 30, 7 and 1 day before they run out."
>
	{#snippet actions()}
		{#if editor}
			<button class="btn-secondary" disabled={checking} onclick={check}>
				<Icon name="refresh" size={14} class={checking ? 'animate-spin' : ''} />
				{checking ? 'Checking…' : 'Check now'}
			</button>
		{/if}
	{/snippet}
</PageHeader>

{#if data.expiry.items.length === 0}
	<div class="card flex flex-col items-center px-6 py-16 text-center">
		<span class="grid size-12 place-items-center rounded-full bg-sunken text-muted"><Icon name="shield" size={22} /></span>
		<h2 class="mt-4 text-[15px] font-semibold">{checking ? 'Looking things up…' : 'Nothing to watch yet'}</h2>
		<p class="mt-1 max-w-sm text-muted">
			Add a website or DNS monitor and heron picks up its domain and certificate here on its own.
		</p>
	</div>
{:else}
	{#if soon > 0}
		<p class="mb-4 flex items-center gap-2 text-[14px] text-ink-2">
			<span class="sq" style="--c: var(--color-degraded); --s: 8px"></span>
			{soon === 1 ? 'One runs out' : `${soon} run out`} within 30 days.
		</p>
	{/if}
	<div class="card overflow-x-auto">
		<table class="w-full">
			<thead class="border-b border-line bg-sunken/70">
				<tr>
					<th class="th">Name</th>
					<th class="th">Expires</th>
					<th class="th hidden md:table-cell">Registrar or issuer</th>
					<th class="th hidden lg:table-cell">Used by</th>
					<th class="th hidden sm:table-cell">Checked</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each data.expiry.items as item (item.kind + item.host)}
					<tr>
						<td class="px-4 py-3.5">
							<div class="flex items-center gap-3">
								<span class="grid size-8 shrink-0 place-items-center rounded-lg bg-sunken text-muted">
									<Icon name={item.kind === 'cert' ? 'key' : 'link'} size={14} />
								</span>
								<div class="min-w-0">
									<p class="truncate font-mono text-[13.5px]">{host(item)}</p>
									<p class="text-[12px] text-faint">{item.kind === 'cert' ? 'Certificate' : 'Domain'}</p>
								</div>
							</div>
						</td>
						<td class="px-4 py-3.5">
							<p class="flex items-center gap-2 text-[14px] font-medium" style="color: {tone(item)}">
								<span class="sq" style="--c: {tone(item)}; --s: 7px"></span>
								{left(item)}
							</p>
							{#if item.error}
								<p class="mt-0.5 max-w-xs truncate text-[12px] text-faint" title={item.error}>{item.error}</p>
							{:else if item.expires_at}
								<p class="mt-0.5 text-[12px] text-faint">{date(item.expires_at)}</p>
							{/if}
						</td>
						<td class="hidden px-4 py-3.5 text-[13px] text-muted md:table-cell">{item.issuer || ''}</td>
						<td class="hidden px-4 py-3.5 lg:table-cell">
							<div class="flex flex-wrap gap-1.5">
								{#each item.monitors.slice(0, 3) as m (m.id)}
									<a href="/admin/monitors/{m.id}" class="chip py-0.5 text-muted ring-line hover:text-ink">{m.name}</a>
								{/each}
								{#if item.monitors.length > 3}<span class="text-[12px] text-faint">+{item.monitors.length - 3}</span>{/if}
							</div>
						</td>
						<td class="hidden px-4 py-3.5 text-[13px] whitespace-nowrap text-faint sm:table-cell">{ago(item.checked_at)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
