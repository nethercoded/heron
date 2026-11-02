(() => {
	const script = document.currentScript;
	if (!script) return;

	const origin = new URL(script.src).origin;
	const position = script.dataset.position || 'bottom-right';
	const showUpcoming = script.dataset.upcoming === 'true';
	const interval = Math.max(Number(script.dataset.refresh) || 60, 15) * 1000;
	const storageKey = 'heron-widget-dismissed';

	const palette = {
		critical: '#dc2626',
		major: '#dc2626',
		minor: '#d97706',
		maintenance: '#2563eb'
	};

	const host = document.createElement('div');
	host.setAttribute('data-heron-widget', '');
	const root = host.attachShadow({ mode: 'open' });

	const anchor = {
		'bottom-right': 'bottom:16px;right:16px;',
		'bottom-left': 'bottom:16px;left:16px;',
		'top-right': 'top:16px;right:16px;',
		'top-left': 'top:16px;left:16px;'
	}[position] || 'bottom:16px;right:16px;';

	root.innerHTML = `
		<style>
			:host { all: initial; }
			.stack { position: fixed; ${anchor} z-index: 2147483000; display: flex; flex-direction: column; gap: 8px; width: min(360px, calc(100vw - 32px)); font: 13px/1.45 Inter, ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif; }
			.card { position: relative; display: flex; gap: 10px; padding: 12px 36px 12px 14px; background: #fff; color: #0f172a; border: 1px solid #e2e8f0; border-left: 4px solid var(--tone); border-radius: 8px; box-shadow: 0 10px 30px -10px rgba(15, 23, 42, .25); text-decoration: none; animation: in .2s ease-out; }
			.card:hover { border-color: #cbd5e1; border-left-color: var(--tone); }
			.dot { flex: none; width: 8px; height: 8px; margin-top: 6px; border-radius: 50%; background: var(--tone); box-shadow: 0 0 0 4px color-mix(in srgb, var(--tone) 18%, transparent); }
			.title { font-weight: 600; }
			.meta { color: #64748b; font-size: 12px; }
			.close { position: absolute; top: 8px; right: 8px; width: 22px; height: 22px; border: 0; border-radius: 4px; background: transparent; color: #94a3b8; font-size: 16px; line-height: 1; cursor: pointer; }
			.close:hover { background: #f1f5f9; color: #0f172a; }
			@media (prefers-color-scheme: dark) {
				.card { background: #111827; color: #f1f5f9; border-color: #1f2937; }
				.meta { color: #94a3b8; }
				.close:hover { background: #1f2937; color: #f1f5f9; }
			}
			@keyframes in { from { opacity: 0; transform: translateY(6px); } }
		</style>
		<div class="stack" role="status" aria-live="polite"></div>
	`;
	const stack = root.querySelector('.stack');

	const dismissed = () => {
		try {
			return JSON.parse(sessionStorage.getItem(storageKey) || '[]');
		} catch {
			return [];
		}
	};

	const dismiss = (key) => {
		try {
			sessionStorage.setItem(storageKey, JSON.stringify([...dismissed(), key]));
		} catch {}
	};

	const when = (iso) => {
		const minutes = Math.round((new Date(iso).getTime() - Date.now()) / 60000);
		const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
		if (Math.abs(minutes) < 60) return rtf.format(minutes, 'minute');
		if (Math.abs(minutes) < 1440) return rtf.format(Math.round(minutes / 60), 'hour');
		return rtf.format(Math.round(minutes / 1440), 'day');
	};

	const render = (summary) => {
		const hidden = dismissed();
		const items = summary.items.filter((item) => (item.active || showUpcoming) && !hidden.includes(item.kind + item.title + item.starts_at));
		stack.replaceChildren(
			...items.map((item) => {
				const key = item.kind + item.title + item.starts_at;
				const card = document.createElement('a');
				card.className = 'card';
				card.href = new URL(item.url, origin).href;
				card.target = '_blank';
				card.rel = 'noopener';
				card.style.setProperty('--tone', palette[item.severity] || palette.major);

				const label = item.kind === 'maintenance' ? (item.active ? 'Maintenance in progress' : `Scheduled maintenance ${when(item.starts_at)}`) : `Started ${when(item.starts_at)}`;
				card.innerHTML = '<span class="dot"></span><span><div class="title"></div><div class="meta"></div></span><button class="close" type="button" aria-label="Dismiss">×</button>';
				card.querySelector('.title').textContent = item.title;
				card.querySelector('.meta').textContent = `${label} · ${summary.title} status`;
				card.querySelector('.close').addEventListener('click', (event) => {
					event.preventDefault();
					event.stopPropagation();
					dismiss(key);
					card.remove();
				});
				return card;
			})
		);
	};

	const refresh = async () => {
		try {
			const res = await fetch(`${origin}/api/summary`, { credentials: 'omit' });
			if (res.ok) render(await res.json());
		} catch {}
	};

	const mount = () => {
		document.body.appendChild(host);
		refresh();
		setInterval(refresh, interval);
	};

	if (document.body) mount();
	else document.addEventListener('DOMContentLoaded', mount, { once: true });
})();
