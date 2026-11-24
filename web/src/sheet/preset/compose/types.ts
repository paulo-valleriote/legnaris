import type { PresetHeaderProps, PresetLayoutBlock, PresetSectionNav } from '../types';

export interface PresetIdentityHeader {
  readonly name: string;
  readonly subtitle?: string;
}

export interface PresetStatPill {
  readonly path: string;
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
}

export interface PresetAbility {
  readonly score: string;
  readonly modifier: string;
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

export type PresetPiece =
  | PresetStatPillsPiece
  | PresetPipsPiece
  | PresetAbilityGridPiece
  | PresetSaveListPiece
  | PresetSkillListPiece
  | PresetHitPointsPiece
  | PresetConditionsPiece
  | PresetRecordTablePiece
  | PresetResetBarPiece
  | PresetRollModePiece;

export type PresetPieceKind = PresetPiece['kind'];

export interface PresetConfig {
  readonly templateId: string;
  readonly header: PresetIdentityHeader;
  readonly pieces: readonly PresetPiece[];
  readonly resetActions?: readonly string[];
  readonly layout?: readonly PresetLayoutBlock[];
  readonly narrowLayout?: readonly PresetLayoutBlock[];
  readonly sectionNav?: PresetSectionNav;
}

export type PresetHeaderLevel = PresetHeaderProps['level'];
