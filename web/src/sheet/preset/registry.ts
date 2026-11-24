import type { TemplateDetail } from '../../api/models/template';
import { presetFromConfig } from './compose/adapter';
import { fifthStatBlockConfig } from './compose/configs/fifth_stat_block';
import type { PresetConfig } from './compose/types';
import { fantasyPreset } from './fantasy';
import type { SheetPreset } from './types';

export const PRESET_CONFIGS: readonly PresetConfig[] = [fifthStatBlockConfig];

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
