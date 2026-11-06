<script lang="ts">
	import type { Evidence } from '#lib/types.ts';
	import Icon from './Icon.svelte';

	let { evidence }: { evidence: Evidence } = $props();

	const rep = $derived(evidence.report);
	const res = $derived(rep.response);
	const timing = $derived(res?.timing);
	const phases = $derived(
		timing
			? [
					{ label: 'DNS', ms: timing.dns_ms, color: 'var(--color-faint)' },
					{ label: 'Connect', ms: timing.connect_ms, color: 'var(--color-accent)' },
					{ label: 'TLS', ms: timing.tls_ms, color: 'var(--color-degraded)' },
					{
						label: 'Waiting',
						ms: Math.max(0, timing.first_byte_ms - timing.dns_ms - timing.connect_ms - timing.tls_ms),
						color: 'var(--color-ink-2)'
					},
					{ label: 'Download', ms: Math.max(0, timing.total_ms - timing.first_byte_ms), color: 'var(--color-up)' }
				].filter((p) => p.ms > 0)
			: []
	);
	const total = $derived(
		Math.max(
			1,
			phases.reduce((s, p) => s + p.ms, 0)
		)
	);

	let open = $state<string | null>(null);

	const time = (iso: string) =>
		new Date(iso).toLocaleString(undefined, {
			day: 'numeric',
			month: 'short',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		});
	const day = (iso: string) =>
		new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
	const tone = (bad: boolean) => (bad ? 'var(--color-down)' : 'var(--color-up)');

	const facts = $derived.by(() => {
		const out: { label: string; value: string; bad: boolean }[] = [];
		if (res) {
			if (res.error) out.push({ label: 'Request', value: res.error, bad: true });
			else out.push({ label: 'Answer', value: res.status ?? '', bad: (res.code ?? 0) >= 400 });
		}
		if (rep.connect)
			out.push({
				label: 'Port',
				value: rep.connect.error ?? `open, ${rep.connect.took_ms} ms`,
				bad: !!rep.connect.error
			});
		if (rep.dns)
			out.push({ label: 'DNS', value: rep.dns.error ?? (rep.dns.addresses ?? []).join(', '), bad: !!rep.dns.error });
		if (rep.tls)
			out.push({
				label: 'Certificate',
				value: rep.tls.valid ? `valid, ${rep.tls.version ?? ''}` : (rep.tls.error ?? 'invalid'),
				bad: !rep.tls.valid
			});
		return out;
	});
</script>

{#snippet toggle(id: string, label: string, hint = '')}
	<button
		type="button"
		class="flex w-full items-center justify-between gap-3 py-3 text-start text-[13.5px] font-medium hover:text-ink {open === id ? 'text-ink' : 'text-ink-2'}"
		aria-expanded={open === id}
		onclick={() => (open = open === id ? null : id)}
	>
		<span>{label}{#if hint}<span class="ms-2 font-normal text-faint">{hint}</span>{/if}</span>
		<Icon name="chevron" size={14} class="transition-transform {open === id ? 'rotate-90' : ''}" />
	</button>
{/snippet}

<section class="card overflow-hidden">
	<div class="flex flex-wrap items-baseline justify-between gap-2 border-b border-line px-5 py-4">
		<div class="min-w-0">
			<h2 class="font-semibold">What heron saw</h2>
			<p class="truncate text-[13px] text-muted">{evidence.monitor_name} · <span class="font-mono">{rep.target}</span></p>
		</div>
		<span class="text-[12.5px] text-faint">{time(evidence.at)}</span>
	</div>

	<div class="space-y-4 px-5 py-4">
		{#if rep.reason}
			<p class="rounded-ctl bg-sunken px-3.5 py-2.5 font-mono text-[12.5px] text-ink-2">{rep.reason}</p>
		{/if}

		<dl class="grid gap-x-6 gap-y-2.5 sm:grid-cols-2">
			{#each facts as fact (fact.label)}
				<div class="min-w-0">
					<dt class="text-[11.5px] text-faint">{fact.label}</dt>
					<dd class="flex items-center gap-2 text-[13.5px]">
						<span class="sq" style="--c: {tone(fact.bad)}; --s: 7px"></span>
						<span class="truncate" title={fact.value}>{fact.value}</span>
					</dd>
				</div>
			{/each}
		</dl>

		{#if phases.length > 0 && timing}
			<div>
				<div class="mb-1.5 flex items-baseline justify-between text-[11.5px] text-faint">
					<span>Where the time went</span>
					<span class="font-mono">{timing.total_ms} ms</span>
				</div>
				<div class="flex h-2.5 overflow-hidden rounded-full bg-sunken">
					{#each phases as p (p.label)}
						<span style="width: {(p.ms / total) * 100}%; background: {p.color}" title="{p.label} {p.ms} ms"></span>
					{/each}
				</div>
				<div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[12px] text-muted">
					{#each phases as p (p.label)}
						<span class="flex items-center gap-1.5"><span class="sq" style="--c: {p.color}; --s: 6px"></span>{p.label} <span class="font-mono text-faint">{p.ms} ms</span></span>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<div class="divide-y divide-line border-t border-line px-5">
		{#if evidence.image}
			{@render toggle('image', 'Screenshot', 'how the page looked')}
			{#if open === 'image'}
				<div class="pb-4">
					<img src="/api/admin/evidence/{evidence.id}/image" alt="The page when it went down" class="w-full rounded-ctl border border-line" loading="lazy" />
				</div>
			{/if}
		{/if}

		{#if res && !res.error}
			{@render toggle('response', 'Response', res.remote ? `from ${res.remote}` : '')}
			{#if open === 'response'}
				<div class="space-y-3 pb-4">
					{#if res.redirect}<p class="text-[13px] text-muted">Redirects to <span class="font-mono text-ink-2">{res.redirect}</span></p>{/if}
					<table class="w-full text-[12.5px]">
						<tbody class="divide-y divide-line">
							{#each res.headers ?? [] as h (h.name)}
								<tr>
									<td class="w-1/3 py-1.5 pe-3 align-top font-mono text-faint">{h.name}</td>
									<td class="py-1.5 font-mono break-all text-ink-2">{h.value}</td>
								</tr>
							{/each}
						</tbody>
					</table>
					{#if res.body}
						<pre class="max-h-72 overflow-auto rounded-ctl bg-canvas p-3 font-mono text-[12px] leading-5 whitespace-pre-wrap text-ink-2">{res.body}{res.cut ? '\n…' : ''}</pre>
					{/if}
				</div>
			{/if}
		{/if}

		{#if rep.dns}
			{@render toggle('dns', 'DNS', `${rep.dns.took_ms} ms`)}
			{#if open === 'dns'}
				<div class="space-y-1 pb-4 font-mono text-[12.5px] text-ink-2">
					<p><span class="text-faint">host</span> {rep.dns.host}</p>
					{#if rep.dns.cname}<p><span class="text-faint">cname</span> {rep.dns.cname}</p>{/if}
					{#each rep.dns.addresses ?? [] as a (a)}<p><span class="text-faint">address</span> {a}</p>{/each}
					{#if rep.dns.error}<p class="text-down">{rep.dns.error}</p>{/if}
				</div>
			{/if}
		{/if}

		{#if rep.tls?.chain?.length}
			{@render toggle('tls', 'Certificates', `${rep.tls.chain.length} in the chain`)}
			{#if open === 'tls'}
				<ol class="space-y-2 pb-4 text-[12.5px]">
					{#each rep.tls.chain as c, i (i)}
						<li class="rounded-ctl bg-canvas px-3 py-2">
							<p class="font-medium text-ink">{c.subject || '(no name)'}</p>
							<p class="text-muted">Issued by {c.issuer} · valid until {day(c.not_after)}</p>
							{#if c.names?.length}<p class="mt-0.5 truncate font-mono text-faint">{c.names.join(', ')}</p>{/if}
						</li>
					{/each}
				</ol>
			{/if}
		{/if}

		{#if rep.route}
			{@render toggle('route', 'Network route', rep.route.tool)}
			{#if open === 'route'}
				<pre class="mb-4 max-h-72 overflow-auto rounded-ctl bg-canvas p-3 font-mono text-[12px] leading-5 text-ink-2">{rep.route.output || rep.route.error}</pre>
			{/if}
		{/if}
	</div>

	{#if rep.notes?.length}
		<div class="border-t border-line px-5 py-3 text-[12px] text-faint">
			{#each rep.notes as note (note)}<p>{note}</p>{/each}
		</div>
	{/if}
</section>
