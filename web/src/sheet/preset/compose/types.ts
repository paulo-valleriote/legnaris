import type { MessageKey } from '../../../l10n/strings';
import type { PresetHeaderProps, PresetLayoutBlock, PresetSectionNav } from '../types';

export interface PresetIdentityHeader {
  readonly name: string;
  readonly subtitle?: string;
  readonly trailing?: string;
}

export interface PresetStatPill {
  readonly path: string;
  readonly row?: string;
}

export interface PresetStatPillsPiece {
  readonly kind: 'statPills';
  readonly key: string;
  readonly pills: readonly PresetStatPill[];
}

export interface PresetPipsPiece {
  readonly kind: 'pips';
  readonly key: string;
  readonly count: string;
  readonly max: string | number;
  readonly heading?: MessageKey;
  readonly label?: MessageKey;
  readonly critical?: boolean;
}

export interface PresetSpellSlotLevel {
  readonly level: number;
  readonly count: string;
  readonly max: string;
}

export interface PresetSpellSlotsPiece {
  readonly kind: 'spellSlots';
  readonly key: string;
  readonly levels: readonly PresetSpellSlotLevel[];
  readonly pact?: { readonly count: string; readonly max: string };
}

export interface PresetAbility {
  readonly score: string;
  readonly modifier: string;
  readonly row?: string;
}

export interface PresetAbilityGridPiece {
  readonly kind: 'abilityGrid';
  readonly key: string;
  readonly section: string;
  readonly abilities: readonly PresetAbility[];
}

export interface PresetSave {
  readonly proficiency: string;
  readonly total: string;
  readonly row?: string;
}

export interface PresetSaveListPiece {
  readonly kind: 'saveList';
  readonly key: string;
  readonly section: string;
  readonly saves: readonly PresetSave[];
}

export interface PresetSkill {
  readonly rank: string;
  readonly total: string;
  readonly ability?: string;
  readonly row?: string;
}

export interface PresetSkillListPiece {
  readonly kind: 'skillList';
  readonly key: string;
  readonly section: string;
  readonly max: string | number;
  readonly skills: readonly PresetSkill[];
}

export interface PresetHitPointsPiece {
  readonly kind: 'hitPoints';
  readonly key: string;
  readonly current: string;
  readonly max: string | number;
  readonly temp?: string;
  readonly downNotice?: MessageKey;
}

export interface PresetConditionFlag {
  readonly path: string;
}

export interface PresetExhaustionTrack {
  readonly level: string;
  readonly max: string | number;
  readonly penalty?: string;
}

export interface PresetConditionsPiece {
  readonly kind: 'conditions';
  readonly key: string;
  readonly conditions: readonly PresetConditionFlag[];
  readonly exhaustion?: PresetExhaustionTrack;
}

export interface PresetRecordTablePiece {
  readonly kind: 'recordTable';
  readonly key: string;
  readonly list: string;
  readonly columns?: readonly string[];
  readonly playColumns?: readonly string[];
  readonly nameColumn?: string;
  readonly rollColumn?: string;
  readonly damageColumn?: string;
}

export interface PresetResetBarPiece {
  readonly kind: 'resetBar';
  readonly key: string;
  readonly actions: readonly string[];
}

export interface PresetRollModePiece {
  readonly kind: 'rollMode';
  readonly key: string;
}

export interface PresetTextField {
  readonly path: string;
}

export interface PresetTextFieldsPiece {
  readonly kind: 'textFields';
  readonly key: string;
  readonly fields: readonly PresetTextField[];
}

export interface PresetFieldsPiece {
  readonly kind: 'fields';
  readonly key: string;
  readonly title: MessageKey;
  readonly fields: readonly PresetTextField[];
  readonly summary?: string;
}

export interface PresetDiceFormulasPiece {
  readonly kind: 'diceFormulas';
  readonly key: string;
}

export type PresetPiece =
  | PresetStatPillsPiece
  | PresetPipsPiece
  | PresetSpellSlotsPiece
  | PresetAbilityGridPiece
  | PresetSaveListPiece
  | PresetSkillListPiece
  | PresetHitPointsPiece
  | PresetConditionsPiece
  | PresetRecordTablePiece
  | PresetResetBarPiece
  | PresetRollModePiece
  | PresetTextFieldsPiece
  | PresetFieldsPiece
  | PresetDiceFormulasPiece;

export type PresetPieceKind = PresetPiece['kind'];

export interface PresetConfig {
  readonly templateId: string;
  readonly header: PresetIdentityHeader;
  readonly pieces: readonly PresetPiece[];
  readonly layout?: readonly PresetLayoutBlock[];
  readonly narrowLayout?: readonly PresetLayoutBlock[];
  readonly sectionNav?: PresetSectionNav;
}

export type PresetHeaderLevel = PresetHeaderProps['level'];
