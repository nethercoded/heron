<script lang="ts">
	import { theme } from '#lib/theme.svelte.ts';
	import Icon from './Icon.svelte';
</script>

<button type="button" class="btn-ghost size-9 px-0" onclick={() => theme.toggle()} aria-label="Toggle dark mode">
	<Icon name={theme.dark ? 'sun' : 'moon'} />
</button>
