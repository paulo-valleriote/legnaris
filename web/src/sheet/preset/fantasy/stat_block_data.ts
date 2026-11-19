import type { SheetKind } from '../../../api/models/plan';
import type { MessageKey } from '../../../l10n/strings';
import { parseClampedInt } from './parse_number';

export const STAT_BLOCK_TEMPLATE_ID = 'fantasy_v1';

export const INITIATIVE_MIN = -5;

export const INITIATIVE_MAX = 20;

const STAT_BLOCK_KIND_LABELS: Readonly<Record<'npc' | 'enemy', MessageKey>> = {
  npc: 'sheet.play.kind.npc',
  enemy: 'sheet.play.kind.enemy',
};

export type StatBlockKind = keyof typeof STAT_BLOCK_KIND_LABELS;

export function isStatBlockKind(kind: SheetKind | undefined): kind is StatBlockKind {
  return typeof kind === 'string' && kind in STAT_BLOCK_KIND_LABELS;
}

export function usesStatBlock(templateId: string, kind: SheetKind | undefined): boolean {
  return templateId === STAT_BLOCK_TEMPLATE_ID && isStatBlockKind(kind);
}

export function statBlockKindLabelKey(kind: SheetKind | undefined): MessageKey | undefined {
  return isStatBlockKind(kind) ? STAT_BLOCK_KIND_LABELS[kind] : undefined;
}

export function parseInitiative(text: string): number | undefined {
  return parseClampedInt(text, INITIATIVE_MIN, INITIATIVE_MAX);
}
