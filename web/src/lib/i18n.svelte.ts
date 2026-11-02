import { createContext } from 'svelte';
import type { Impact, IncidentStatus, Overall, Range, Status } from './types.ts';

export type Locale = 'en' | 'ru' | 'he';

type Plural = Partial<Record<Intl.LDMLPluralRule, string>> & { other: string };
type Message = string | Plural;

export const locales: { code: Locale; name: string }[] = [
	{ code: 'en', name: 'English' },
	{ code: 'ru', name: 'Русский' },
	{ code: 'he', name: 'עברית' }
];

const en = {
	'nav.status': 'Status',
	'nav.history': 'History',
	'nav.language': 'Language',
	'live.on': 'Live',
	'live.off': 'Offline',
	'live.connected': 'Receiving live updates',
	'live.reconnecting': 'Reconnecting',

	'status.up': 'Operational',
	'status.degraded': 'Degraded',
	'status.down': 'Down',
	'status.pending': 'Pending',
	'status.paused': 'Paused',
	'status.maintenance': 'Maintenance',

	'word.up': 'Operational',
	'word.degraded': 'Degraded',
	'word.down': 'Outage',
	'word.pending': 'Waiting',
	'word.paused': 'Paused',
	'word.maintenance': 'Maintenance',

	'overall.operational': 'All systems operational',
	'overall.operational.detail': 'Every service is responding normally.',
	'overall.degraded': 'Degraded performance',
	'overall.degraded.detail': 'Some services are slower than usual.',
	'overall.partial_outage': 'Partial system outage',
	'overall.partial_outage.detail': 'Some services are unavailable.',
	'overall.major_outage': 'Major system outage',
	'overall.major_outage.detail': 'Most services are unavailable.',
	'overall.maintenance': 'Scheduled maintenance',
	'overall.maintenance.detail': 'Planned work is in progress.',

	'impact.minor': 'Minor',
	'impact.major': 'Major',
	'impact.critical': 'Critical',
	'impact.maintenance': 'Maintenance',

	'incident.investigating': 'Investigating',
	'incident.identified': 'Identified',
	'incident.monitoring': 'Monitoring',
	'incident.resolved': 'Resolved',

	'page.unavailable': 'Status is unavailable right now',
	'page.later': 'Please try again in a moment.',
	'page.retry': 'Retry',
	'hero.uptime': { one: 'Uptime, {n} day', other: 'Uptime, {n} days' },
	'hero.online': 'Services online',
	'hero.checked': 'Checked',
	'section.now': 'Happening now',
	'section.planned': 'Planned work',
	'group.services': 'Services',
	'group.healthy': 'All operational',
	'group.empty': 'No services are being monitored yet.',
	'week.all': 'All history',
	'week.today': 'Today',
	'week.quiet': 'No incidents',

	'last.hours': { one: 'Last hour', other: 'Last {n} hours' },
	'last.days': { one: 'Last day', other: 'Last {n} days' },

	'history.title': 'Incident history',
	'history.lead': 'Everything that went wrong over the past year, and what we did about it.',
	'history.empty': 'No incidents reported',
	'history.empty.lead': 'Nothing has gone wrong in the past year.',

	'monitor.back': 'All services',
	'monitor.for': 'for {time}',
	'monitor.response': 'Response time',
	'monitor.average': 'Average {latency} · last check {ago}',
	'monitor.recent': 'Recent checks',
	'monitor.latest': '{latency} latest',
	'monitor.history': 'Uptime history',
	'monitor.calendar': 'Uptime calendar',
	'monitor.incidents': 'Incidents',
	'monitor.quiet': { one: 'No incidents in the last day.', other: 'No incidents in the last {n} days.' },

	'card.started': 'Started {date}',
	'card.resolved': 'Resolved in {time}',
	'card.ongoing': 'Ongoing for {time}',
	'card.earlier': { one: 'View {n} earlier update', other: 'View {n} earlier updates' },
	'card.affected': 'Affected services',

	'maintenance.active': 'In progress',
	'maintenance.starts': 'Starts {when}',
	'maintenance.daily': 'Repeats daily',
	'maintenance.weekly': 'Repeats weekly',

	'bar.empty': 'No data recorded',
	'bar.uptime': 'Uptime',
	'bar.failed': 'Failed checks',
	'bar.degraded': 'Degraded checks',
	'bar.total': '{percent} uptime',

	'calendar.year': 'uptime over the last year',
	'calendar.clean': {
		one: 'of {n} tracked day without failures',
		other: 'of {n} tracked days without failures'
	},
	'calendar.since': 'Monitoring since {date}',
	'calendar.empty': 'no data',
	'calendar.failed': { one: '{down} failed of {n} check', other: '{down} failed of {n} checks' },
	'calendar.hint': 'Hover a day for details',
	'calendar.worse': 'Worse',
	'calendar.better': 'Better',

	'chart.empty': 'Not enough data yet',
	'chart.label': 'Response time chart',
	'chart.average': 'Average',
	'chart.peak': 'Peak',
	'chart.failed': 'Failed',
	'chart.of': '{down} of {n}',

	'time.never': 'never',
	'time.now': 'just now',
	'unit.ms': 'ms',
	'unit.s': 's'
} satisfies Record<string, Message>;

export type MessageKey = keyof typeof en;
type Dictionary = Record<MessageKey, Message>;

const ru: Dictionary = {
	'nav.status': 'Статус',
	'nav.history': 'История',
	'nav.language': 'Язык',
	'live.on': 'Онлайн',
	'live.off': 'Офлайн',
	'live.connected': 'Обновления приходят в реальном времени',
	'live.reconnecting': 'Переподключаемся',

	'status.up': 'Работает',
	'status.degraded': 'Замедлен',
	'status.down': 'Недоступен',
	'status.pending': 'Ожидание',
	'status.paused': 'На паузе',
	'status.maintenance': 'Обслуживание',

	'word.up': 'Работает',
	'word.degraded': 'Замедлен',
	'word.down': 'Сбой',
	'word.pending': 'Ожидание',
	'word.paused': 'На паузе',
	'word.maintenance': 'Работы',

	'overall.operational': 'Все системы работают',
	'overall.operational.detail': 'Все сервисы отвечают в штатном режиме.',
	'overall.degraded': 'Снижена производительность',
	'overall.degraded.detail': 'Некоторые сервисы работают медленнее обычного.',
	'overall.partial_outage': 'Частичный сбой',
	'overall.partial_outage.detail': 'Часть сервисов недоступна.',
	'overall.major_outage': 'Серьёзный сбой',
	'overall.major_outage.detail': 'Большинство сервисов недоступно.',
	'overall.maintenance': 'Плановое обслуживание',
	'overall.maintenance.detail': 'Идут плановые работы.',

	'impact.minor': 'Незначительный',
	'impact.major': 'Серьёзный',
	'impact.critical': 'Критический',
	'impact.maintenance': 'Обслуживание',

	'incident.investigating': 'Выясняем',
	'incident.identified': 'Причина найдена',
	'incident.monitoring': 'Наблюдаем',
	'incident.resolved': 'Решено',

	'page.unavailable': 'Статус сейчас недоступен',
	'page.later': 'Попробуйте ещё раз чуть позже.',
	'page.retry': 'Повторить',
	'hero.uptime': {
		one: 'Доступность за {n} день',
		few: 'Доступность за {n} дня',
		other: 'Доступность за {n} дней'
	},
	'hero.online': 'Сервисов онлайн',
	'hero.checked': 'Проверено',
	'section.now': 'Происходит сейчас',
	'section.planned': 'Плановые работы',
	'group.services': 'Сервисы',
	'group.healthy': 'Всё работает',
	'group.empty': 'Пока ни один сервис не отслеживается.',
	'week.all': 'Вся история',
	'week.today': 'Сегодня',
	'week.quiet': 'Без инцидентов',

	'last.hours': {
		one: 'Последние {n} час',
		few: 'Последние {n} часа',
		other: 'Последние {n} часов'
	},
	'last.days': {
		one: 'Последние {n} день',
		few: 'Последние {n} дня',
		other: 'Последние {n} дней'
	},

	'history.title': 'История инцидентов',
	'history.lead': 'Всё, что шло не так за последний год, и что мы с этим сделали.',
	'history.empty': 'Инцидентов не было',
	'history.empty.lead': 'За последний год ничего не ломалось.',

	'monitor.back': 'Все сервисы',
	'monitor.for': 'уже {time}',
	'monitor.response': 'Время ответа',
	'monitor.average': 'В среднем {latency} · последняя проверка {ago}',
	'monitor.recent': 'Последние проверки',
	'monitor.latest': 'последний ответ: {latency}',
	'monitor.history': 'История доступности',
	'monitor.calendar': 'Календарь доступности',
	'monitor.incidents': 'Инциденты',
	'monitor.quiet': {
		one: 'За последние {n} день инцидентов не было.',
		few: 'За последние {n} дня инцидентов не было.',
		other: 'За последние {n} дней инцидентов не было.'
	},

	'card.started': 'Начало: {date}',
	'card.resolved': 'Решено за {time}',
	'card.ongoing': 'Длится уже {time}',
	'card.earlier': {
		one: 'Показать ещё {n} обновление',
		few: 'Показать ещё {n} обновления',
		other: 'Показать ещё {n} обновлений'
	},
	'card.affected': 'Затронутые сервисы',

	'maintenance.active': 'Идут работы',
	'maintenance.starts': 'Начнётся {when}',
	'maintenance.daily': 'Каждый день',
	'maintenance.weekly': 'Каждую неделю',

	'bar.empty': 'Данных нет',
	'bar.uptime': 'Доступность',
	'bar.failed': 'Неудачные проверки',
	'bar.degraded': 'Медленные проверки',
	'bar.total': 'доступность {percent}',

	'calendar.year': 'доступности за последний год',
	'calendar.clean': {
		one: 'из {n} отслеживаемого дня без сбоев',
		other: 'из {n} отслеживаемых дней без сбоев'
	},
	'calendar.since': 'Наблюдаем с {date}',
	'calendar.empty': 'нет данных',
	'calendar.failed': {
		one: 'сбоев: {down} из {n} проверки',
		other: 'сбоев: {down} из {n} проверок'
	},
	'calendar.hint': 'Наведите на день, чтобы увидеть подробности',
	'calendar.worse': 'Хуже',
	'calendar.better': 'Лучше',

	'chart.empty': 'Пока недостаточно данных',
	'chart.label': 'График времени ответа',
	'chart.average': 'Среднее',
	'chart.peak': 'Пик',
	'chart.failed': 'Сбои',
	'chart.of': '{down} из {n}',

	'time.never': 'никогда',
	'time.now': 'только что',
	'unit.ms': 'мс',
	'unit.s': 'с'
};

const he: Dictionary = {
	'nav.status': 'סטטוס',
	'nav.history': 'היסטוריה',
	'nav.language': 'שפה',
	'live.on': 'חי',
	'live.off': 'מנותק',
	'live.connected': 'מתקבלים עדכונים בזמן אמת',
	'live.reconnecting': 'מתחבר מחדש',

	'status.up': 'תקין',
	'status.degraded': 'איטי',
	'status.down': 'מושבת',
	'status.pending': 'ממתין',
	'status.paused': 'מושהה',
	'status.maintenance': 'בתחזוקה',

	'word.up': 'תקין',
	'word.degraded': 'איטי',
	'word.down': 'תקלה',
	'word.pending': 'ממתין',
	'word.paused': 'מושהה',
	'word.maintenance': 'תחזוקה',

	'overall.operational': 'כל המערכות פועלות כרגיל',
	'overall.operational.detail': 'כל השירותים מגיבים כרגיל.',
	'overall.degraded': 'ירידה בביצועים',
	'overall.degraded.detail': 'חלק מהשירותים איטיים מהרגיל.',
	'overall.partial_outage': 'תקלה חלקית',
	'overall.partial_outage.detail': 'חלק מהשירותים אינם זמינים.',
	'overall.major_outage': 'תקלה נרחבת',
	'overall.major_outage.detail': 'רוב השירותים אינם זמינים.',
	'overall.maintenance': 'תחזוקה מתוכננת',
	'overall.maintenance.detail': 'מתבצעות עבודות מתוכננות.',

	'impact.minor': 'קלה',
	'impact.major': 'חמורה',
	'impact.critical': 'קריטית',
	'impact.maintenance': 'תחזוקה',

	'incident.investigating': 'בבדיקה',
	'incident.identified': 'הגורם זוהה',
	'incident.monitoring': 'במעקב',
	'incident.resolved': 'נפתרה',

	'page.unavailable': 'הסטטוס אינו זמין כרגע',
	'page.later': 'נסו שוב בעוד רגע.',
	'page.retry': 'ניסיון חוזר',
	'hero.uptime': {
		one: 'זמינות ביום האחרון',
		two: 'זמינות ביומיים האחרונים',
		other: 'זמינות ב־{n} הימים האחרונים'
	},
	'hero.online': 'שירותים פעילים',
	'hero.checked': 'נבדק',
	'section.now': 'מתרחש עכשיו',
	'section.planned': 'עבודות מתוכננות',
	'group.services': 'שירותים',
	'group.healthy': 'הכול תקין',
	'group.empty': 'עדיין אין שירותים במעקב.',
	'week.all': 'כל ההיסטוריה',
	'week.today': 'היום',
	'week.quiet': 'אין תקלות',

	'last.hours': { one: 'השעה האחרונה', two: 'השעתיים האחרונות', other: '{n} השעות האחרונות' },
	'last.days': { one: 'היום האחרון', two: 'היומיים האחרונים', other: '{n} הימים האחרונים' },

	'history.title': 'היסטוריית תקלות',
	'history.lead': 'כל מה שהשתבש בשנה האחרונה, ומה עשינו כדי לטפל בזה.',
	'history.empty': 'לא דווחו תקלות',
	'history.empty.lead': 'שום דבר לא השתבש בשנה האחרונה.',

	'monitor.back': 'כל השירותים',
	'monitor.for': 'כבר {time}',
	'monitor.response': 'זמן תגובה',
	'monitor.average': 'ממוצע {latency} · בדיקה אחרונה {ago}',
	'monitor.recent': 'בדיקות אחרונות',
	'monitor.latest': 'אחרונה: {latency}',
	'monitor.history': 'היסטוריית זמינות',
	'monitor.calendar': 'לוח זמינות',
	'monitor.incidents': 'תקלות',
	'monitor.quiet': {
		one: 'לא היו תקלות ביום האחרון.',
		two: 'לא היו תקלות ביומיים האחרונים.',
		other: 'לא היו תקלות ב־{n} הימים האחרונים.'
	},

	'card.started': 'התחלה: {date}',
	'card.resolved': 'נפתרה תוך {time}',
	'card.ongoing': 'נמשכת כבר {time}',
	'card.earlier': {
		one: 'הצגת עדכון קודם אחד',
		two: 'הצגת שני עדכונים קודמים',
		other: 'הצגת {n} עדכונים קודמים'
	},
	'card.affected': 'שירותים מושפעים',

	'maintenance.active': 'מתבצעת כעת',
	'maintenance.starts': 'תתחיל {when}',
	'maintenance.daily': 'חוזרת מדי יום',
	'maintenance.weekly': 'חוזרת מדי שבוע',

	'bar.empty': 'אין נתונים',
	'bar.uptime': 'זמינות',
	'bar.failed': 'בדיקות שנכשלו',
	'bar.degraded': 'בדיקות איטיות',
	'bar.total': 'זמינות {percent}',

	'calendar.year': 'זמינות בשנה האחרונה',
	'calendar.clean': {
		one: 'מתוך יום מעקב אחד ללא תקלות',
		other: 'מתוך {n} ימי מעקב ללא תקלות'
	},
	'calendar.since': 'במעקב מאז {date}',
	'calendar.empty': 'אין נתונים',
	'calendar.failed': {
		one: '{down} נכשלו מתוך בדיקה אחת',
		other: '{down} נכשלו מתוך {n} בדיקות'
	},
	'calendar.hint': 'העבירו את העכבר מעל יום כדי לראות פרטים',
	'calendar.worse': 'גרוע',
	'calendar.better': 'טוב',

	'chart.empty': 'אין עדיין מספיק נתונים',
	'chart.label': 'תרשים זמן תגובה',
	'chart.average': 'ממוצע',
	'chart.peak': 'שיא',
	'chart.failed': 'כשלים',
	'chart.of': '{down} מתוך {n}',

	'time.never': 'אף פעם',
	'time.now': 'עכשיו',
	'unit.ms': 'ms',
	'unit.s': 's'
};

const dictionaries: Record<Locale, Dictionary> = { en, ru, he };

const steps: [Intl.RelativeTimeFormatUnit, number][] = [
	['year', 31_536_000],
	['month', 2_592_000],
	['week', 604_800],
	['day', 86_400],
	['hour', 3_600],
	['minute', 60],
	['second', 1]
];

function formatters(locale: Locale) {
	const unit = (name: string, display: 'narrow' | 'short' = 'narrow') =>
		new Intl.NumberFormat(locale, { style: 'unit', unit: name, unitDisplay: display });
	return {
		plural: new Intl.PluralRules(locale),
		relative: new Intl.RelativeTimeFormat(locale, { numeric: 'auto', style: 'long' }),
		daysAgo: new Intl.RelativeTimeFormat(locale, { numeric: 'always', style: 'long' }),
		dateTime: new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' }),
		date: new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }),
		month: new Intl.DateTimeFormat(locale, { month: 'long', year: 'numeric' }),
		time: new Intl.DateTimeFormat(locale, { timeStyle: 'short' }),
		dayMonth: new Intl.DateTimeFormat(locale, { month: 'short', day: 'numeric' }),
		weekday: new Intl.DateTimeFormat(locale, { weekday: 'short' }),
		monthShort: new Intl.DateTimeFormat(locale, { month: 'short' }),
		units: {
			day: unit('day'),
			hour: unit('hour'),
			minute: unit('minute'),
			second: unit('second')
		},
		range: { h: unit('hour'), d: unit('day') },
		compact: new Intl.NumberFormat(locale, { maximumFractionDigits: 2, useGrouping: false }),
		numbers: new Map<number, Intl.NumberFormat>(),
		percents: new Map<number, Intl.NumberFormat>()
	};
}

type Formats = ReturnType<typeof formatters>;

const cache = new Map<Locale, Formats>();

function formatsFor(locale: Locale) {
	let formats = cache.get(locale);
	if (!formats) {
		formats = formatters(locale);
		cache.set(locale, formats);
	}
	return formats;
}

function memo(map: Map<number, Intl.NumberFormat>, digits: number, create: () => Intl.NumberFormat) {
	let format = map.get(digits);
	if (!format) {
		format = create();
		map.set(digits, format);
	}
	return format;
}

function parseDay(date: string) {
	const [y, m, d] = date.split('-').map(Number);
	return new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1);
}

export function isLocale(value: unknown): value is Locale {
	return value === 'en' || value === 'ru' || value === 'he';
}

const storageKey = 'heron:locale';

export function preferredLocale(): Locale {
	try {
		const saved = localStorage.getItem(storageKey);
		if (isLocale(saved)) return saved;
	} catch {}
	const language = (typeof navigator === 'undefined' ? '' : navigator.language).toLowerCase();
	if (language.startsWith('ru')) return 'ru';
	if (language.startsWith('he') || language.startsWith('iw')) return 'he';
	return 'en';
}

export class I18n {
	locale = $state<Locale>('en');

	constructor(locale: Locale = 'en') {
		this.locale = locale;
	}

	get dir() {
		return this.locale === 'he' ? 'rtl' : 'ltr';
	}

	get #formats() {
		return formatsFor(this.locale);
	}

	set(locale: Locale) {
		this.locale = locale;
		try {
			localStorage.setItem(storageKey, locale);
		} catch {}
	}

	t(key: MessageKey, vars: Record<string, string | number> = {}) {
		const message = dictionaries[this.locale][key];
		const text = typeof message === 'string' ? message : this.#plural(message, Number(vars.n ?? 0));
		return text.replace(/\{(\w+)\}/g, (match, name: string) => (name in vars ? String(vars[name]) : match));
	}

	#plural(forms: Plural, n: number) {
		return forms[this.#formats.plural.select(n)] ?? forms.other;
	}

	status(status: Status) {
		return this.t(`status.${status}` as const);
	}

	word(status: Status) {
		return this.t(`word.${status}` as const);
	}

	overall(overall: Overall) {
		return {
			title: this.t(`overall.${overall}` as const),
			detail: this.t(`overall.${overall}.detail` as const)
		};
	}

	impact(impact: Impact) {
		return this.t(`impact.${impact}` as const);
	}

	incident(status: IncidentStatus) {
		return this.t(`incident.${status}` as const);
	}

	range(range: Range) {
		const value = Number.parseInt(range, 10);
		return range.endsWith('h') ? this.#formats.range.h.format(value) : this.#formats.range.d.format(value);
	}

	ago(iso: string | undefined, now = Date.now()) {
		if (!iso) return this.t('time.never');
		const seconds = Math.round((new Date(iso).getTime() - now) / 1000);
		if (Math.abs(seconds) < 5) return this.t('time.now');
		for (const [unit, size] of steps) {
			if (Math.abs(seconds) >= size || unit === 'second') {
				return this.#formats.relative.format(Math.round(seconds / size), unit);
			}
		}
		return '';
	}

	daysAgo(days: number) {
		return this.#formats.daysAgo.format(-days, 'day');
	}

	duration(ms: number) {
		const { day, hour, minute, second } = this.#formats.units;
		const s = Math.max(0, Math.round(ms / 1000));
		const d = Math.floor(s / 86_400);
		const h = Math.floor((s % 86_400) / 3_600);
		const m = Math.floor((s % 3_600) / 60);
		if (d > 0) return h > 0 ? `${day.format(d)} ${hour.format(h)}` : day.format(d);
		if (h > 0) return m > 0 ? `${hour.format(h)} ${minute.format(m)}` : hour.format(h);
		if (m > 0) return minute.format(m);
		return second.format(s);
	}

	between(from: string, to?: string) {
		return this.duration((to ? new Date(to) : new Date()).getTime() - new Date(from).getTime());
	}

	number(value: number, digits = 0) {
		const format = memo(
			this.#formats.numbers,
			digits,
			() =>
				new Intl.NumberFormat(this.locale, {
					minimumFractionDigits: digits,
					maximumFractionDigits: digits,
					useGrouping: false
				})
		);
		return format.format(value);
	}

	percent(ratio: number | null | undefined, digits = 2) {
		if (ratio == null) return '—';
		const exact = ratio * 100 === 100;
		const places = exact ? 0 : digits;
		const format = memo(
			this.#formats.percents,
			places,
			() =>
				new Intl.NumberFormat(this.locale, {
					style: 'percent',
					minimumFractionDigits: places,
					maximumFractionDigits: places
				})
		);
		return format.format(exact ? 1 : Math.min(ratio, 0.9999));
	}

	latency(ms: number | undefined) {
		if (!ms) return '—';
		return this.#isolate(
			ms >= 1000
				? `${this.number(ms / 1000, 2)} ${this.t('unit.s')}`
				: `${this.number(Math.round(ms))} ${this.t('unit.ms')}`
		);
	}

	#isolate(text: string) {
		return this.dir === 'rtl' ? `⁦${text}⁩` : text;
	}

	axis(ms: number) {
		const { compact } = this.#formats;
		return ms >= 1000 ? `${compact.format(ms / 1000)}${this.t('unit.s')}` : `${compact.format(ms)}${this.t('unit.ms')}`;
	}

	dateTime(iso: string) {
		return this.#formats.dateTime.format(new Date(iso));
	}

	date(iso: string) {
		return this.#formats.date.format(new Date(iso));
	}

	month(iso: string) {
		return this.#formats.month.format(new Date(iso));
	}

	time(iso: string) {
		return this.#formats.time.format(new Date(iso));
	}

	day(date: string) {
		return this.#formats.date.format(parseDay(date));
	}

	dayMonth(date: Date) {
		return this.#formats.dayMonth.format(date);
	}

	weekday(date: Date) {
		return this.#formats.weekday.format(date);
	}

	monthShort(date: Date) {
		return this.#formats.monthShort.format(date);
	}
}

const english = new I18n();

const [getI18n, setI18n, hasI18n] = createContext<I18n>();

export function provideI18n(locale: Locale = preferredLocale()) {
	return setI18n(new I18n(locale));
}

export function useI18n() {
	return hasI18n() ? getI18n() : english;
}
