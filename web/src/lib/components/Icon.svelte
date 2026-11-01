<script lang="ts" module>
	const paths = {
		check: 'M20 6 9 17l-5-5',
		x: 'M18 6 6 18M6 6l12 12',
		alert: 'M12 9v4m0 4h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z',
		info: 'M12 16v-4m0-4h.01M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0Z',
		wrench:
			'M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.8-3.8a6 6 0 0 1-7.9 7.9l-6.9 6.9a2.1 2.1 0 0 1-3-3l6.9-6.9a6 6 0 0 1 7.9-7.9l-3.8 3.8Z',
		pause: 'M10 4H6v16h4zM18 4h-4v16h4z',
		play: 'm6 3 14 9-14 9V3z',
		refresh: 'M21 12a9 9 0 0 1-15.7 6M3 12A9 9 0 0 1 18.7 6M21 3v5h-5M3 21v-5h5',
		plus: 'M12 5v14M5 12h14',
		trash: 'M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6',
		edit: 'M12 20h9M16.4 3.6a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z',
		external: 'M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6',
		sun: 'M12 16a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
		moon: 'M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z',
		chevron: 'm9 18 6-6-6-6',
		back: 'm15 18-6-6 6-6',
		bell: 'M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9M10.3 21a1.9 1.9 0 0 0 3.4 0',
		copy: 'M8 8h12v12H8zM4 16V4h12',
		logout: 'M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9',
		activity: 'M22 12h-4l-3 9L9 3l-3 9H2',
		settings:
			'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6ZM19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z',
		incident: 'M12 2 2 7v6c0 5 4.3 8.7 10 9 5.7-.3 10-4 10-9V7l-10-5ZM12 8v4m0 4h.01',
		monitor: 'M3 4h18v12H3zM8 20h8M12 16v4',
		menu: 'M4 6h16M4 12h16M4 18h16',
		search: 'm21 21-4.3-4.3M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16Z',
		key: 'm15.5 7.5 3 3L22 7l-3-3m-3.5 3.5L19 4m-3.5 3.5-4.3 4.3a5.5 5.5 0 1 1-1.4-1.4L14 6',
		link: 'M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7',
		send: 'm22 2-7 20-4-9-9-4Z M22 2 11 13'
	} as const;

	export type IconName = keyof typeof paths;
</script>

<script lang="ts">
	let { name, size = 16, class: className = '' }: { name: IconName; size?: number; class?: string } = $props();
</script>

<svg
	xmlns="http://www.w3.org/2000/svg"
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	stroke-linecap="round"
	stroke-linejoin="round"
	class="shrink-0 {className}"
	aria-hidden="true"
>
	<path d={paths[name]} />
</svg>
