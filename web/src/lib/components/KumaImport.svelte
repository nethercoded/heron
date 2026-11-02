<script lang="ts">
	import { ApiError } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import { invalidate } from '$app/navigation';
	import Icon from './Icon.svelte';

	type Plan = {
		channels: { name: string; kind: string }[];
		monitors: { name: string; kind: string; target: string; group?: string; paused: boolean; notes?: string[] }[];
		skipped: { name: string; kind: string; reason: string }[];
	};

	let file = $state<File | null>(null);
	let plan = $state<Plan | null>(null);
	let busy = $state(false);
	let done = $state(false);
	let input = $state<HTMLInputElement>();
	let over = $state(false);

	async function send(path: string): Promise<Plan> {
		const body = new FormData();
		body.set('file', file as File);
		const res = await fetch(`/api${path}`, { method: 'POST', body, credentials: 'same-origin' });
		const data = await res.json().catch(() => null);
		if (!res.ok) throw new ApiError(res.status, data?.code ?? 'unknown', data?.message ?? res.statusText);
		return data as Plan;
	}

	async function pick(next: File | null | undefined) {
		if (!next) return;
		file = next;
		plan = null;
		done = false;
		busy = true;
		try {
			plan = await send('/admin/import/kuma/preview');
		} catch (err) {
			file = null;
			toast.error(err instanceof Error ? err.message : 'Could not read the file');
		} finally {
			busy = false;
			if (input) input.value = '';
		}
	}

	async function run() {
		if (!plan) return;
		busy = true;
		try {
			plan = await send('/admin/import/kuma');
			done = true;
			toast.success(`Imported ${plan.monitors.length} monitors, heron is checking them now`);
			await invalidate('heron:counts');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Import failed');
		} finally {
			busy = false;
		}
	}

	function reset() {
		file = null;
		plan = null;
		done = false;
	}
</script>

<section class="card space-y-5 p-5 sm:p-6">
	<div>
		<h2 class="font-semibold">Move from Uptime Kuma</h2>
		<p class="text-sm text-muted">
			Monitors, groups and Telegram, Discord or webhook notifications come over in one go. Push monitors keep their tokens, so your cron
			jobs only need the new address.
		</p>
	</div>

	{#if !plan}
		<button
			type="button"
			class="flex w-full flex-col items-center gap-2 rounded-ctl border border-dashed px-6 py-8 text-center transition-colors {over ? 'border-ink bg-sunken' : 'border-line-strong hover:border-faint'}"
			disabled={busy}
			onclick={() => input?.click()}
			ondragover={(e) => {
				e.preventDefault();
				over = true;
			}}
			ondragleave={() => (over = false)}
			ondrop={(e) => {
				e.preventDefault();
				over = false;
				pick(e.dataTransfer?.files[0]);
			}}
		>
			<Icon name="upload" size={20} class="text-muted" />
			<span class="text-[14px] font-medium">{busy ? `Reading ${file?.name}…` : 'Drop the backup here or choose a file'}</span>
			<span class="text-[12.5px] text-faint">
				The JSON from Settings → Backup → Export in Kuma 1.x, or <code class="kbd">kuma.db</code> from its data folder
			</span>
		</button>
		<input bind:this={input} type="file" accept=".json,.db,.sqlite,application/json" class="hidden" onchange={(e) => pick(e.currentTarget.files?.[0])} />
	{:else}
		<div class="rounded-ctl border border-line">
			<div class="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3 text-sm">
				<span class="min-w-0 truncate font-mono text-[13px]">{file?.name}</span>
				<span class="text-muted">
					{plan.monitors.length} monitors · {plan.channels.length} channels{plan.skipped.length ? ` · ${plan.skipped.length} skipped` : ''}
				</span>
			</div>
			<ul class="max-h-72 divide-y divide-line overflow-y-auto text-[13px]">
				{#each plan.channels as ch (ch.name + ch.kind)}
					<li class="flex items-center gap-3 px-4 py-2.5">
						<Icon name="bell" size={14} class="text-faint" />
						<span class="min-w-0 flex-1 truncate">{ch.name}</span>
						<span class="font-mono text-[11px] text-faint uppercase">{ch.kind}</span>
					</li>
				{/each}
				{#each plan.monitors as m, i (i)}
					<li class="px-4 py-2.5">
						<div class="flex items-center gap-3">
							<span class="sq" style="--c: var(--color-{m.paused ? 'paused' : 'up'}); --s: 8px"></span>
							<span class="min-w-0 flex-1 truncate">
								{m.name}
								{#if m.group}<span class="text-faint"> · {m.group}</span>{/if}
							</span>
							<span class="hidden max-w-[45%] truncate font-mono text-[12px] text-faint sm:inline">{m.target}</span>
							<span class="w-16 text-end font-mono text-[11px] text-faint uppercase">{m.kind}</span>
						</div>
						{#each m.notes ?? [] as note (note)}
							<p class="mt-1 ps-5 text-[12px] text-degraded">{note}</p>
						{/each}
					</li>
				{/each}
				{#each plan.skipped as s, i (i)}
					<li class="flex items-start gap-3 px-4 py-2.5 text-faint">
						<Icon name="x" size={14} class="mt-0.5" />
						<span class="min-w-0 flex-1"><span class="text-muted">{s.name || 'Unnamed'}</span> · {s.reason}</span>
					</li>
				{/each}
			</ul>
		</div>
		<div class="flex justify-end gap-2">
			{#if done}
				<button type="button" class="btn-secondary" onclick={reset}>Import another file</button>
				<a href="/admin" class="btn-primary">See monitors</a>
			{:else}
				<button type="button" class="btn-ghost" disabled={busy} onclick={reset}>Cancel</button>
				<button type="button" class="btn-primary" disabled={busy || (plan.monitors.length === 0 && plan.channels.length === 0)} onclick={run}>
					{busy ? 'Importing…' : `Import ${plan.monitors.length} monitors`}
				</button>
			{/if}
		</div>
	{/if}
</section>
