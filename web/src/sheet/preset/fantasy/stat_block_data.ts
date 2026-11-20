import type { SheetKind } from '../../../api/models/plan';
import type { MessageKey } from '../../../l10n/strings';

const STAT_BLOCK_KIND_LABELS: Readonly<Record<'npc' | 'enemy', MessageKey>> = {
  npc: 'kind.npc',
  enemy: 'kind.enemy',
};

export type StatBlockKind = keyof typeof STAT_BLOCK_KIND_LABELS;

export interface StatBlockIdentity {
  readonly ancestry: string;
  readonly origin: string;
  readonly level: number | undefined;
}

export const STAT_PROSE_SEPARATOR = ' \u00b7 ';
export const PASSIVE_AWARENESS_PATH = 'derived.passiveAwareness';

export interface StatMovementTexts {
  readonly speed: string;
  readonly passive: string;
  readonly feet: (count: number) => string;
}

export function statMovementParts(
  speed: number | undefined,
  passiveAwareness: number | undefined,
  texts: StatMovementTexts,
): readonly string[] {
  const parts: string[] = [];
  if (speed !== undefined) {
    parts.push(`${texts.speed} ${texts.feet(speed)}`);
  }
  if (passiveAwareness !== undefined) {
    parts.push(`${texts.passive} ${passiveAwareness}`);
  }
  return parts;
}

export function statProseText(parts: readonly string[]): string | undefined {
  return parts.length === 0 ? undefined : parts.join(STAT_PROSE_SEPARATOR);
}

export function statBlockAbilityWord(abbreviation: string): string {
  return abbreviation.charAt(0).toUpperCase() + abbreviation.slice(1).toLowerCase();
}

export function isStatBlockKind(kind: SheetKind | undefined): kind is StatBlockKind {
  return typeof kind === 'string' && Object.hasOwn(STAT_BLOCK_KIND_LABELS, kind);
}

export function statBlockKindLabelKey(kind: SheetKind | undefined): MessageKey | undefined {
  return isStatBlockKind(kind) ? STAT_BLOCK_KIND_LABELS[kind] : undefined;
}

export function statBlockSubtitleParts(
  identity: StatBlockIdentity,
  levelText: (level: number) => string,
): readonly string[] {
  const parts: string[] = [];
  const ancestry = identity.ancestry.trim();
  const origin = identity.origin.trim();
  if (ancestry !== '') {
    parts.push(ancestry);
  }
  if (origin !== '') {
    parts.push(origin);
  }
  if (identity.level !== undefined) {
    parts.push(levelText(identity.level));
  }
  return parts;
}
