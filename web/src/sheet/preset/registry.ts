import type { TemplateDetail } from '../../api/models/template';
import { presetFromConfig } from './compose/adapter';
import { fifthCharacterConfig } from './compose/configs/fifth_character';
import type { PresetConfig } from './compose/types';
import { fantasyPreset } from './fantasy';
import type { SheetPreset } from './types';

export const PRESET_CONFIGS: readonly PresetConfig[] = [fifthCharacterConfig];

export function presetFor(
  template: TemplateDetail,
  configs: readonly PresetConfig[] = PRESET_CONFIGS,
): SheetPreset | undefined {
  if (template.id === fantasyPreset.templateId) {
    return fantasyPreset;
  }
  const config = configs.find((entry) => entry.templateId === template.id);
  return config === undefined ? undefined : presetFromConfig(config);
}
