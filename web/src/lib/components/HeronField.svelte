<script lang="ts" module>
	const GAP = 6;
	const DOT = 2.2;
	const REACH = 72;
	const SPRING = 0.009;
	const DAMP = 0.9;
	const MAX_OFF = 30;
	const FIG_W = 420;
	const WATER = 524;

	type Ripple = { x: number; y: number; t0: number };
	type Point = { x: number; y: number };

	function drawSilhouette(ctx: CanvasRenderingContext2D) {
		ctx.fillStyle = '#fff';
		ctx.strokeStyle = '#fff';
		ctx.lineCap = 'round';
		ctx.lineJoin = 'round';

		ctx.beginPath();
		ctx.ellipse(298, 282, 98, 36, 0.16, 0, Math.PI * 2);
		ctx.fill();

		ctx.beginPath();
		ctx.moveTo(360, 262);
		ctx.lineTo(414, 304);
		ctx.lineTo(356, 304);
		ctx.closePath();
		ctx.fill();

		ctx.lineWidth = 24;
		ctx.beginPath();
		ctx.moveTo(176, 112);
		ctx.bezierCurveTo(204, 138, 200, 172, 183, 198);
		ctx.bezierCurveTo(166, 224, 176, 250, 228, 266);
		ctx.stroke();

		ctx.beginPath();
		ctx.ellipse(172, 106, 27, 18, -0.12, 0, Math.PI * 2);
		ctx.fill();

		ctx.beginPath();
		ctx.moveTo(156, 96);
		ctx.lineTo(50, 120);
		ctx.lineTo(156, 118);
		ctx.closePath();
		ctx.fill();

		ctx.lineWidth = 6;
		ctx.beginPath();
		ctx.moveTo(176, 94);
		ctx.bezierCurveTo(200, 80, 228, 80, 250, 94);
		ctx.stroke();

		ctx.lineWidth = 9;
		ctx.beginPath();
		ctx.moveTo(272, 310);
		ctx.lineTo(266, 420);
		ctx.lineTo(270, WATER);
		ctx.moveTo(300, 310);
		ctx.lineTo(312, 414);
		ctx.lineTo(304, WATER);
		ctx.stroke();

		ctx.lineWidth = 6;
		ctx.beginPath();
		ctx.moveTo(252, WATER);
		ctx.lineTo(322, WATER);
		ctx.stroke();
	}
</script>

<script lang="ts">
	let { water = '#4cc3f5', class: className = '' }: { water?: string; class?: string } = $props();

	let canvas = $state<HTMLCanvasElement>();

	$effect(() => {
		if (!canvas) return;
		const el = canvas;
		const ctx = el.getContext('2d');
		if (!ctx) return;

		const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
		const fg = getComputedStyle(el).getPropertyValue('--color-ink').trim() || '#ffffff';
		const tint = () => water;

		let w = 0;
		let h = 0;
		let cols = 0;
		let rows = 0;
		let mask = new Uint8Array(0);
		let phase = new Float32Array(0);
		let offX = new Float32Array(0);
		let offY = new Float32Array(0);
		let velX = new Float32Array(0);
		let velY = new Float32Array(0);
		let waterY = 0;
		let waterFrom = 0;
		let waterTo = 0;
		let feet: [number, number][] = [];
		let ripples: Ripple[] = [];
		let nextRipple = 0;
		let mouse: Point = { x: -1e4, y: -1e4 };
		let prev: Point = { x: -1e4, y: -1e4 };
		let raf = 0;
		const start = performance.now();
		let last = start;

		const build = () => {
			const rect = el.getBoundingClientRect();
			w = Math.max(1, Math.floor(rect.width));
			h = Math.max(1, Math.floor(rect.height));
			const dpr = Math.min(window.devicePixelRatio || 1, 2);
			el.width = w * dpr;
			el.height = h * dpr;
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

			cols = Math.ceil(w / GAP) + 1;
			rows = Math.ceil(h / GAP) + 1;
			const cells = cols * rows;
			phase = Float32Array.from({ length: cells }, () => Math.random() * Math.PI * 2);
			offX = new Float32Array(cells);
			offY = new Float32Array(cells);
			velX = new Float32Array(cells);
			velY = new Float32Array(cells);

			const short = h < 480;
			waterY = h * (short ? 0.86 : 0.82);
			const scale = Math.min((waterY - h * (short ? 0.06 : 0.4)) / (WATER - 80), (w * 0.5) / FIG_W);
			const figW = FIG_W * scale;
			const ox = w - figW - w * 0.06;
			const oy = waterY - WATER * scale;
			waterFrom = ox - 70 * scale;
			waterTo = ox + figW + 60 * scale;

			const off = document.createElement('canvas');
			off.width = w;
			off.height = h;
			const octx = off.getContext('2d', { willReadFrequently: true });
			if (!octx) return;
			octx.setTransform(scale, 0, 0, scale, ox, oy);
			drawSilhouette(octx);
			const data = octx.getImageData(0, 0, w, h).data;
			mask = new Uint8Array(w * h);
			for (let i = 0; i < w * h; i++) mask[i] = (data[i * 4 + 3] ?? 0) > 96 ? 1 : 0;

			feet = [
				[ox + 270 * scale, waterY],
				[ox + 304 * scale, waterY]
			];
		};

		const inside = (x: number, y: number) => {
			const xi = x | 0;
			const yi = y | 0;
			return xi >= 0 && yi >= 0 && xi < w && yi < h && mask[yi * w + xi] === 1;
		};

		const stir = (a: Point, b: Point, dt: number) => {
			const sx = b.x - a.x;
			const sy = b.y - a.y;
			const len = Math.hypot(sx, sy);
			if (len < 0.5) return;
			const ux = sx / len;
			const uy = sy / len;
			const speed = Math.min(len / dt, 50);
			const c0 = Math.max(0, Math.floor((Math.min(a.x, b.x) - REACH) / GAP));
			const c1 = Math.min(cols - 1, Math.ceil((Math.max(a.x, b.x) + REACH) / GAP));
			const r0 = Math.max(0, Math.floor((Math.min(a.y, b.y) - REACH) / GAP));
			const r1 = Math.min(rows - 1, Math.ceil((Math.max(a.y, b.y) + REACH) / GAP));
			for (let row = r0; row <= r1; row++) {
				const y = row * GAP;
				for (let col = c0; col <= c1; col++) {
					const x = col * GAP;
					const p = Math.max(0, Math.min(1, ((x - a.x) * sx + (y - a.y) * sy) / (len * len)));
					const dx = x - (a.x + sx * p);
					const dy = y - (a.y + sy * p);
					const d = Math.hypot(dx, dy);
					if (d >= REACH) continue;
					const f = (1 - d / REACH) ** 2 * speed;
					const nx = d > 0.01 ? dx / d : -uy;
					const ny = d > 0.01 ? dy / d : ux;
					const i = row * cols + col;
					velX[i] = (velX[i] ?? 0) + (nx * 0.09 + ux * 0.04) * f;
					velY[i] = (velY[i] ?? 0) + (ny * 0.09 + uy * 0.04) * f;
				}
			}
		};

		const settle = (dt: number) => {
			const k = SPRING * dt;
			const damp = DAMP ** dt;
			for (let i = 0; i < offX.length; i++) {
				let ox = offX[i] ?? 0;
				let oy = offY[i] ?? 0;
				let vx = velX[i] ?? 0;
				let vy = velY[i] ?? 0;
				if (ox === 0 && oy === 0 && vx === 0 && vy === 0) continue;
				vx = (vx - ox * k) * damp;
				vy = (vy - oy * k) * damp;
				ox += vx * dt;
				oy += vy * dt;
				const m = Math.hypot(ox, oy);
				if (m > MAX_OFF) {
					ox *= MAX_OFF / m;
					oy *= MAX_OFF / m;
				}
				if (m < 0.05 && Math.abs(vx) + Math.abs(vy) < 0.02) ox = oy = vx = vy = 0;
				offX[i] = ox;
				offY[i] = oy;
				velX[i] = vx;
				velY[i] = vy;
			}
		};

		const frame = (now: number) => {
			const t = (now - start) / 1000;
			const dt = Math.min(3, (now - last) / 16.67);
			last = now;
			ctx.clearRect(0, 0, w, h);

			if (mouse.x > -1e3 && prev.x > -1e3) stir(prev, mouse, dt);
			prev = mouse;
			settle(dt);

			if (!reduced && t > nextRipple) {
				const f = feet[Math.random() < 0.5 ? 0 : 1];
				if (f) ripples.push({ x: f[0], y: f[1], t0: t });
				nextRipple = t + 2.4 + Math.random() * 2.2;
			}
			ripples = ripples.filter((r) => t - r.t0 < 5);

			const color = tint();
			for (let row = 0; row < rows; row++) {
				const y = row * GAP;
				const below = y > waterY;
				for (let col = 0; col < cols; col++) {
					const x = col * GAP;
					const idx = row * cols + col;
					let alpha = 0.09 + 0.025 * Math.sin(t * 0.8 + (phase[idx] ?? 0));
					let fill = fg;

					if (!below) {
						if (inside(x, y)) alpha = 1;
					} else {
						const depth = y - waterY;
						const wave = Math.sin(depth * 0.09 - t * 1.7 + x * 0.004) * Math.min(6, depth * 0.08);
						if (inside(x + wave, 2 * waterY - y)) {
							alpha = Math.max(0.2, 0.6 - (depth / (h - waterY + 1)) * 0.4);
							fill = color;
						}
						for (const r of ripples) {
							const age = t - r.t0;
							const d = Math.hypot(x - r.x, (y - r.y) * 2.6);
							const band = 1 - Math.abs(d - age * 46) / 7;
							const ring = band > 0 ? (band > 0.4 ? 1 : 0.55) * (1 - age / 5) : 0;
							if (ring > alpha) {
								alpha = ring;
								fill = color;
							}
						}
					}

					const vx = velX[idx] ?? 0;
					const vy = velY[idx] ?? 0;
					const v = Math.abs(vx) + Math.abs(vy);
					if (v > 0.05) {
						alpha += Math.min(0.12, v * 0.04);
						if (below) fill = color;
					}

					if (Math.abs(y - waterY) < GAP / 2 && x > waterFrom && x < waterTo) {
						alpha = Math.max(alpha, 0.7);
						fill = color;
					}

					ctx.globalAlpha = Math.min(1, alpha);
					ctx.fillStyle = fill;
					ctx.fillRect(x + (offX[idx] ?? 0), y + (offY[idx] ?? 0), DOT, DOT);
				}
			}
			ctx.globalAlpha = 1;
			if (!reduced) raf = requestAnimationFrame(frame);
		};

		build();
		raf = requestAnimationFrame(frame);

		const ro = new ResizeObserver(() => {
			cancelAnimationFrame(raf);
			build();
			raf = requestAnimationFrame(frame);
		});
		ro.observe(el);

		const onMove = (e: PointerEvent) => {
			const rect = el.getBoundingClientRect();
			const x = e.clientX - rect.left;
			const y = e.clientY - rect.top;
			mouse = !reduced && x >= 0 && y >= 0 && x <= rect.width && y <= rect.height ? { x, y } : { x: -1e4, y: -1e4 };
		};
		const onLeave = () => {
			mouse = { x: -1e4, y: -1e4 };
		};
		window.addEventListener('pointermove', onMove);
		document.addEventListener('pointerleave', onLeave);

		return () => {
			cancelAnimationFrame(raf);
			ro.disconnect();
			window.removeEventListener('pointermove', onMove);
			document.removeEventListener('pointerleave', onLeave);
		};
	});
</script>

<canvas bind:this={canvas} class={className} aria-hidden="true"></canvas>
