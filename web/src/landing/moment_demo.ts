import type { Sheet } from '../api/models/sheet';
import type { FieldNode, TemplateDetail } from '../api/models/template';

const MOMENT_TEMPLATE_ID = 'landing_moments_v1';

const MOMENT_SHEET_ID = '00000000-0000-7000-8000-000000000430';

function numberLeaf(label: string, defaultValue: number): FieldNode {
  return {
    kind: 'leaf',
    schema: { type: 'number', label, isEditable: true, defaultValue },
  };
}

function group(label: string, children: Record<string, FieldNode>): FieldNode {
  return { kind: 'group', label, children };
}

export const MOMENT_TEMPLATE: TemplateDetail = {
  id: MOMENT_TEMPLATE_ID,
  name: 'Landing moments demo',
  default: false,
  kinds: ['character'],
  fields: {
    identity: group('Identity', {
      level: numberLeaf('Level', 3),
      xp: numberLeaf('Experience points', 1400),
    }),
    combat: group('Combat', {
      hitPoints: numberLeaf('Current hit points', 24),
      hitPointsMax: numberLeaf('Hit point maximum', 30),
      hitPointsTemp: numberLeaf('Temporary hit points', 0),
    }),
    spells: group('Spells', {
      slot1: group('Level 1', {
        total: numberLeaf('Level 1 spell slots', 2),
        expended: numberLeaf('Level 1 spell slots spent', 0),
      }),
    }),
  },
  dice: { section: {}, row: {} },
};

export const MOMENT_SHEET: Sheet = {
  id: MOMENT_SHEET_ID,
  templateId: MOMENT_TEMPLATE_ID,
  kind: 'character',
  name: 'Landing moments',
  data: {
    identity: { level: 3, xp: 1400 },
    combat: { hitPoints: 24, hitPointsMax: 30, hitPointsTemp: 0 },
    spells: { slot1: { total: 2, expended: 0 } },
  },
  dice: { section: {}, row: {} },
  createdAt: '2026-11-24T00:00:00.000Z',
  updatedAt: '2026-11-24T00:00:00.000Z',
};
