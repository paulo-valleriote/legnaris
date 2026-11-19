import type { ComponentType } from 'react';
import type { SheetKind } from '../../api/models/plan';

export interface PresetSection {
  readonly key: string;
  readonly Component: ComponentType;
}

export interface PresetSectionBlock {
  readonly sections: readonly string[];
}

export interface PresetColumnsBlock {
  readonly columns: readonly (readonly string[])[];
}

export type PresetLayoutBlock = PresetSectionBlock | PresetColumnsBlock;

export interface PresetStatBlock {
  readonly when: (kind: SheetKind | undefined) => boolean;
  readonly Header: ComponentType;
  readonly Body: ComponentType;
}

export interface SheetPreset {
  readonly templateId: string;
  readonly Header: ComponentType;
  readonly statBlock?: PresetStatBlock;
  readonly sections: readonly PresetSection[];
  readonly layout?: readonly PresetLayoutBlock[];
}
