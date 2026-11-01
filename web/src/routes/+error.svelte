<script lang="ts">
	import { page } from '$app/state';
</script>

<svelte:head>
	<title>{page.status} · heron</title>
</svelte:head>

<main class="grid min-h-dvh place-items-center px-6">
	<div class="text-center">
		<p class="font-mono text-sm text-faint">{page.status}</p>
		<h1 class="mt-2 text-2xl font-semibold tracking-tight">
			{page.status === 404 ? 'Page not found' : 'Something went wrong'}
		</h1>
		<p class="mt-2 text-muted">{page.error?.message}</p>
		<a href="/" class="btn-secondary mt-6">Back to status</a>
	</div>
</main>
