import type { Sheet } from '../api/models/sheet';
import { valueAtPath } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import type { DemoDerivation } from './demo_store';
import { FIFTH_DEMO_SHEET, FIFTH_DEMO_TEMPLATE } from './fifth_demo_character';
import { templateDerivation } from './template_derivation';

const SLOT_LEVELS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

export const MOMENT_TEMPLATE: TemplateDetail = FIFTH_DEMO_TEMPLATE;

export const MOMENT_SHEET: Sheet = FIFTH_DEMO_SHEET;

export const MOMENT_DERIVATION: DemoDerivation = templateDerivation(FIFTH_DEMO_TEMPLATE);

function readNumber(data: Record<string, unknown>, path: string): number {
  try {
    const value = valueAtPath(data, path);
    return typeof value === 'number' && Number.isFinite(value) ? value : 0;
  } catch {
    return 0;
  }
}

export function longRestCascade(data: Record<string, unknown>): Record<string, unknown> {
  const slots = Object.fromEntries(SLOT_LEVELS.map((level) => [`slot${level}`, { spent: 0 }]));

  return {
    combat: { hp: { current: readNumber(data, 'combat.hp.max'), temp: 0 } },
    spellcasting: slots,
  };
}
