import type { ComponentType } from 'react';

export interface PresetSection {
  readonly key: string;
  readonly Component: ComponentType;
}

export interface SheetPreset {
  readonly templateId: string;
  readonly Header: ComponentType;
  readonly sections: readonly PresetSection[];
}
