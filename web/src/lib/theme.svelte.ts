const key = 'heron:theme';

class Theme {
	dark = $state(typeof document !== 'undefined' && document.documentElement.classList.contains('dark'));

	toggle() {
		this.dark = !this.dark;
		document.documentElement.classList.toggle('dark', this.dark);
		try {
			localStorage.setItem(key, this.dark ? 'dark' : 'light');
		} catch {}
	}
}

export const theme = new Theme();
