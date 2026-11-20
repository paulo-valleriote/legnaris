import type { SheetKind } from '../../../api/models/plan';
import type { MessageKey } from '../../../l10n/strings';

const STAT_BLOCK_KIND_LABELS: Readonly<Record<'npc' | 'enemy', MessageKey>> = {
  npc: 'sheet.play.kind.npc',
  enemy: 'sheet.play.kind.enemy',
};

export type StatBlockKind = keyof typeof STAT_BLOCK_KIND_LABELS;

export function isStatBlockKind(kind: SheetKind | undefined): kind is StatBlockKind {
  return typeof kind === 'string' && Object.hasOwn(STAT_BLOCK_KIND_LABELS, kind);
}

export function statBlockKindLabelKey(kind: SheetKind | undefined): MessageKey | undefined {
  return isStatBlockKind(kind) ? STAT_BLOCK_KIND_LABELS[kind] : undefined;
}
