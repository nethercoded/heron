export function widgetSnippet(origin: string, position = 'bottom-right') {
	return `<script src="${origin}/widget.js" data-position="${position}" async></script>`;
}

export function badgeMarkdown(origin: string, label: string, path: string, link: string) {
	return `[![${label}](${origin}${path})](${origin}${link})`;
}
