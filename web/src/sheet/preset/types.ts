import type { ComponentType } from 'react';

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

export interface SheetPreset {
  readonly templateId: string;
  readonly Header: ComponentType;
  readonly sections: readonly PresetSection[];
  readonly layout?: readonly PresetLayoutBlock[];
}
