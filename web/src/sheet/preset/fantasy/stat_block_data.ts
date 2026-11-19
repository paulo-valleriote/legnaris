import type { SheetKind } from '../../../api/models/plan';
import type { MessageKey } from '../../../l10n/strings';

export const STAT_BLOCK_TEMPLATE_ID = 'fantasy_v1';

export const INITIATIVE_MIN = -5;

export const INITIATIVE_MAX = 20;

export function isStatBlockKind(kind: SheetKind | undefined): kind is 'npc' | 'enemy' {
  return kind === 'npc' || kind === 'enemy';
}

export function usesStatBlock(templateId: string, kind: SheetKind | undefined): boolean {
  return templateId === STAT_BLOCK_TEMPLATE_ID && isStatBlockKind(kind);
}

export function statBlockKindLabelKey(kind: SheetKind | undefined): MessageKey | undefined {
  if (kind === 'npc') {
    return 'sheet.play.kind.npc';
  }
  if (kind === 'enemy') {
    return 'sheet.play.kind.enemy';
  }
  return undefined;
}

export function parseInitiative(text: string): number | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return undefined;
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) {
    return undefined;
  }
  const clamped = Math.min(INITIATIVE_MAX, Math.max(INITIATIVE_MIN, Math.round(parsed)));
  return clamped === 0 ? 0 : clamped;
}
