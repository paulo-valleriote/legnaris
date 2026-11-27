import type { ComponentType } from 'react';
import type { MessageKey } from '../../l10n/strings';

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

export interface PresetHeaderProps {
  readonly level?: 1 | 2;
}

export interface PresetSectionNavTarget {
  readonly label: MessageKey;
  readonly sections: readonly string[];
}

export interface PresetSectionNav {
  readonly after: string;
  readonly targets?: readonly PresetSectionNavTarget[];
}

export interface SheetPreset {
  readonly templateId: string;
  readonly Header: ComponentType<PresetHeaderProps>;
  readonly sections: readonly PresetSection[];
  readonly layout?: readonly PresetLayoutBlock[];
  readonly narrowLayout?: readonly PresetLayoutBlock[];
  readonly sectionNav?: PresetSectionNav;
  readonly columnsClassName?: string;
}
