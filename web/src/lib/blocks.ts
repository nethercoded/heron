import type { Block, BlockType, Group, Tone } from './types.ts';

export const defaultBlocks = (): Block[] => [
	{ id: 'hero', type: 'hero' },
	{ id: 'incidents', type: 'incidents' },
	{ id: 'maintenance', type: 'maintenance' },
	{ id: 'monitors', type: 'monitors' },
	{ id: 'history', type: 'history', days: 7 }
];

export const blockKinds: { type: BlockType; label: string; hint: string; icon: string }[] = [
	{ type: 'hero', label: 'Status header', hint: 'Overall status, uptime and the heron', icon: 'activity' },
	{ type: 'notice', label: 'Notice', hint: 'Your own text: an announcement, a warning, a link', icon: 'info' },
	{ type: 'incidents', label: 'Active incidents', hint: 'Appears only while something is broken', icon: 'incident' },
	{ type: 'maintenance', label: 'Planned work', hint: 'Appears only when maintenance is scheduled', icon: 'wrench' },
	{ type: 'monitors', label: 'Services', hint: 'One group of monitors, or every group left', icon: 'monitor' },
	{ type: 'history', label: 'Recent days', hint: 'A day by day log of incidents', icon: 'history' }
];

export const kindOf = (type: BlockType) =>
	blockKinds.find((k) => k.type === type) ?? (blockKinds[0] as (typeof blockKinds)[number]);

export const toneColor: Record<Tone, string> = {
	info: 'var(--color-accent)',
	success: 'var(--color-up)',
	warning: 'var(--color-degraded)',
	danger: 'var(--color-down)'
};

export function blockGroups(blocks: Block[], block: Block, groups: Group[]): Group[] {
	if (block.group) return groups.filter((g) => g.name === block.group);
	const claimed = new Set(blocks.filter((b) => b.type === 'monitors' && b.group).map((b) => b.group));
	return groups.filter((g) => !claimed.has(g.name));
}

export function historySpan(blocks: Block[]) {
	return Math.max(7, ...blocks.filter((b) => b.type === 'history' && !b.hidden).map((b) => b.days || 7));
}

export function newId(type: BlockType, blocks: Block[]) {
	const taken = new Set(blocks.map((b) => b.id));
	for (let i = 1; ; i++) {
		const id = i === 1 ? type : `${type}-${i}`;
		if (!taken.has(id)) return id;
	}
}

export function blank(type: BlockType, blocks: Block[]): Block {
	const block: Block = { id: newId(type, blocks), type };
	if (type === 'notice')
		return { ...block, tone: 'info', title: 'Heads up', body: 'Tell your users something worth knowing.' };
	if (type === 'history') return { ...block, days: 7 };
	return block;
}
