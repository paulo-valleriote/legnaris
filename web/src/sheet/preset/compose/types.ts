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

export type PresetPiece =
  PresetStatPillsPiece | PresetPipsPiece | PresetHitPointsPiece | PresetConditionsPiece;

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
