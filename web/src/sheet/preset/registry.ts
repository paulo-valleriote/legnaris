import type { TemplateDetail } from '../../api/models/template';
import { fantasyPreset } from './fantasy';
import type { SheetPreset } from './types';

const PRESETS: readonly SheetPreset[] = [fantasyPreset];

export function presetFor(template: TemplateDetail): SheetPreset | undefined {
  return PRESETS.find((preset) => preset.templateId === template.id);
}
