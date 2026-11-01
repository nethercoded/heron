<script lang="ts" module>
	const rows = [
		'...XX......',
		'XXXXX......',
		'....X......',
		'...X.......',
		'...X.......',
		'...XXXX....',
		'....XXXXXX.',
		'.....XXXXXX',
		'......X.X..',
		'......X.X..',
		'......X.X..',
		'.....XX.XX.',
		'..ww.wwwww.w'
	];
</script>

<script lang="ts">
	let { size = 28, water = false, class: className = '' }: { size?: number; water?: boolean; class?: string } = $props();

	const grid = $derived(water ? rows : rows.slice(0, 12));
	const cell = $derived(size / grid.length);
	const dot = $derived(cell * 0.74);
	const width = $derived(12 * cell);
	const cells = $derived(
		grid.flatMap((row, y) => [...row].flatMap((ch, x) => (ch === '.' ? [] : [{ x, y, water: ch === 'w' }])))
	);
</script>

<svg viewBox="0 0 {width.toFixed(2)} {size}" width={width} height={size} class="shrink-0 {className}" aria-hidden="true">
	{#each cells as c (`${c.x}-${c.y}`)}
		<rect
			x={c.x * cell}
			y={c.y * cell}
			width={dot}
			height={dot}
			rx={dot * 0.22}
			fill={c.water ? 'var(--color-accent)' : 'currentColor'}
		/>
	{/each}
</svg>
