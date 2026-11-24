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
