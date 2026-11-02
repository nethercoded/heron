<script lang="ts">
	import { onMount } from 'svelte';
	import { api, message } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import Snippet from './Snippet.svelte';

	let { origin }: { origin: string } = $props();

	let token = $state('');
	let busy = $state(false);

	const target = $derived(new URL(origin));
	const scrape = $derived(`scrape_configs:
  - job_name: heron
    scheme: ${target.protocol.replace(':', '')}
    metrics_path: /metrics
    authorization:
      credentials: ${token}
    static_configs:
      - targets: ['${target.host}']`);

	onMount(async () => {
		try {
			token = (await api.get<{ token: string }>('/admin/metrics')).token;
		} catch (err) {
			toast.error(message(err));
		}
	});

	async function rotate() {
		if (token && !confirm('Issue a new token? Prometheus stops scraping until you update its config.')) return;
		busy = true;
		try {
			token = (await api.post<{ token: string }>('/admin/metrics')).token;
			toast.success('Metrics are on');
		} catch (err) {
			toast.error(message(err));
		} finally {
			busy = false;
		}
	}

	async function disable() {
		if (!confirm('Turn off /metrics?')) return;
		try {
			await api.delete('/admin/metrics');
			token = '';
			toast.success('Metrics are off');
		} catch (err) {
			toast.error(message(err));
		}
	}
</script>

<section class="card space-y-4 p-5 sm:p-6">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="min-w-0">
			<h2 class="font-semibold">Prometheus</h2>
			<p class="text-sm text-muted">
				{token
					? 'heron serves status, response time and uptime of every monitor at /metrics. Point Prometheus at it and build dashboards in Grafana.'
					: 'Expose monitor status, response time and uptime at /metrics for Prometheus and Grafana. Off until you create a token.'}
			</p>
		</div>
		<div class="flex gap-2">
			{#if token}
				<button class="btn-ghost btn-sm" onclick={disable}>Turn off</button>
				<button class="btn-secondary btn-sm" disabled={busy} onclick={rotate}>New token</button>
			{:else}
				<button class="btn-primary btn-sm" disabled={busy} onclick={rotate}>Turn on</button>
			{/if}
		</div>
	</div>
	{#if token}
		<Snippet label="prometheus.yml" code={scrape} />
	{/if}
</section>
