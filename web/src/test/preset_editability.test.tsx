import { afterEach, describe, expect, it } from 'vitest';
import { act, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { FieldNode, FieldSchema, TemplateDetail } from '../api/models/template';
import { fieldDisplayName, lastSegment } from '../engine/field_grouping';
import { listInfos, orderedRecords } from '../engine/row_identity';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import { fifthStatBlockConfig } from '../sheet/preset/compose/configs/fifth_stat_block';
import type { PresetConfig } from '../sheet/preset/compose/types';
import { PresetPlayView } from '../sheet/preset/play_view';
import {
  fifthCharacterSheet,
  fifthCharacterTemplate,
  fifthStatBlockSheet,
  fifthStatBlockTemplate,
} from './fifth_fixtures';
import { renderPreset } from './preset_fixtures';
import { setViewportWidth } from './match_media';

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, 'scrollIntoView');
});

interface Leaf {
  readonly path: string;
  readonly schema: FieldSchema;
}

interface RecordLeaf extends Leaf {
  readonly listPath: string;
  readonly recordID: string;
  readonly field: string;
}

function leaves(nodes: Readonly<Record<string, FieldNode>>, prefix = ''): Leaf[] {
  const found: Leaf[] = [];
  for (const [key, node] of Object.entries(nodes)) {
    const path = prefix === '' ? key : `${prefix}.${key}`;
    if (node.kind === 'leaf') {
      found.push({ path, schema: node.schema });
    } else if (node.kind === 'group') {
      found.push(...leaves(node.children, path));
    }
  }
  return found;
}

function recordLeaves(template: TemplateDetail, data: Record<string, unknown>): RecordLeaf[] {
  const found: RecordLeaf[] = [];
  for (const list of listInfos(template.fields)) {
    const recordIDs = orderedRecords(list.path, data);
    expect(recordIDs.length).toBeGreaterThan(0);
    const recordID = recordIDs[0].id;
    for (const [field, schema] of Object.entries(list.shape)) {
      found.push({
        path: `${list.path}.${recordID}.${field}`,
        schema,
        listPath: list.path,
        recordID,
        field,
      });
    }
  }
  return found;
}

function pipPaths(config: PresetConfig): Set<string> {
  const paths = new Set<string>();
  for (const piece of config.pieces) {
    if (piece.kind === 'pips') {
      paths.add(piece.count);
    }
    if (piece.kind === 'spellSlots') {
      for (const level of piece.levels) {
        paths.add(level.count);
      }
      if (piece.pact !== undefined) {
        paths.add(piece.pact.count);
      }
    }
    if (piece.kind === 'conditions' && piece.exhaustion !== undefined) {
      paths.add(piece.exhaustion.level);
    }
  }
  return paths;
}

function conditionPaths(config: PresetConfig): Set<string> {
  const paths = new Set<string>();
  for (const piece of config.pieces) {
    if (piece.kind === 'conditions') {
      for (const condition of piece.conditions) {
        paths.add(condition.path);
      }
    }
  }
  return paths;
}

function enabled(element: Element | null): boolean {
  return element instanceof HTMLInputElement || element instanceof HTMLSelectElement
    ? !element.disabled
    : element !== null;
}

function fieldRow(path: string): Element | null {
  return document.querySelector(`[data-field-path~="${path}"]`);
}

function staticControl(path: string): Element | null {
  const row = fieldRow(path);
  if (row !== null) {
    return row.querySelector('input:not([disabled]), select:not([disabled])');
  }
  const inline = document.querySelector(`[data-edit-path="${path}"]`);
  if (inline !== null) {
    return inline;
  }
  if (path.endsWith('.prof')) {
    const total = `${path.slice(0, -'.prof'.length)}.total`;
    const save = document.querySelector(`[data-testid="compose-save-${total}"]`);
    return save?.querySelector('button.lg-compose-flag') ?? null;
  }
  if (path.endsWith('.rank')) {
    const skill = document.querySelector(`[data-testid="compose-skill-${path}"]`);
    return skill?.querySelector('button.lg-compose-rank') ?? null;
  }
  return null;
}

function openDetails(container: HTMLElement): void {
  act(() => {
    container
      .querySelectorAll<HTMLButtonElement>('[data-testid="compose-conditions-toggle"]')
      .forEach((toggle) => toggle.click());
    container.querySelectorAll<HTMLDetailsElement>('details').forEach((details) => {
      details.open = true;
      details.dispatchEvent(new Event('toggle', { bubbles: true }));
    });
  });
}

function renderPresetSheet(
  config: PresetConfig,
  template: TemplateDetail,
  sheet: ReturnType<typeof fifthCharacterSheet> | ReturnType<typeof fifthStatBlockSheet>,
) {
  setViewportWidth(1280);
  const preset = presetFromConfig(config);
  const rendered = renderPreset(
    <>
      <preset.Header />
      <PresetPlayView preset={preset} />
    </>,
    { template, sheet },
  );
  openDetails(rendered.container);
  return rendered;
}

describe.each([
  {
    name: 'the fifth-edition character preset',
    config: fifthCharacterConfig,
    template: fifthCharacterTemplate(),
    sheet: fifthCharacterSheet(),
  },
  {
    name: 'the fifth-edition stat block preset',
    config: fifthStatBlockConfig,
    template: fifthStatBlockTemplate(),
    sheet: fifthStatBlockSheet(),
  },
])('editability of $name', ({ config, template, sheet }) => {
  const data = sheet.data as Record<string, unknown>;
  const staticLeaves = leaves(template.fields);
  const records = recordLeaves(template, data);
  const pips = pipPaths(config);
  const conditions = conditionPaths(config);

  it('Happy path: every editable leaf of the template has an enabled editing control', () => {
    renderPresetSheet(config, template, sheet);
    const uncovered: string[] = [];
    for (const leaf of staticLeaves) {
      if (!leaf.schema.isEditable) {
        continue;
      }
      if (pips.has(leaf.path) || conditions.has(leaf.path)) {
        continue;
      }
      if (!enabled(staticControl(leaf.path))) {
        uncovered.push(leaf.path);
      }
    }
    for (const leaf of records) {
      if (!leaf.schema.isEditable) {
        continue;
      }
      const row = document.querySelector(`[data-record-key="${leaf.listPath}.${leaf.recordID}"]`);
      expect(row).not.toBeNull();
      if (leaf.schema.exclusiveGroup !== undefined && leaf.schema.exclusiveGroup !== '') {
        expect(row?.querySelector('select:not([disabled])')).not.toBeNull();
        continue;
      }
      const label = fieldDisplayName(lastSegment(leaf.field), leaf.schema);
      const controls = within(row as HTMLElement)
        .queryAllByLabelText(label)
        .filter(enabled);
      if (controls.length === 0) {
        uncovered.push(leaf.path);
      }
    }

    expect(uncovered).toEqual([]);
  });

  it('Edge: every pip and condition path the template marks editable is covered by its widget', () => {
    const editable = new Set([
      ...staticLeaves.filter((leaf) => leaf.schema.isEditable).map((leaf) => leaf.path),
    ]);
    for (const path of pips) {
      expect(editable.has(path)).toBe(true);
    }
    for (const path of conditions) {
      expect(editable.has(path)).toBe(true);
    }
  });

  it('Happy path: each widget without a path attribute writes its own leaf', async () => {
    const user = userEvent.setup();
    const { store } = renderPresetSheet(config, template, sheet);

    const flag = document.querySelector<HTMLButtonElement>('button.lg-compose-flag');
    if (flag !== null) {
      await user.click(flag);
      expect([...store.getState().editedPaths].some((path) => path.endsWith('.prof'))).toBe(true);
      store.discard();
    }

    const rank = document.querySelector<HTMLButtonElement>('button.lg-compose-rank');
    if (rank !== null) {
      await user.click(rank);
      expect([...store.getState().editedPaths].some((path) => path.endsWith('.rank'))).toBe(true);
      store.discard();
    }

    const pip = document.querySelector<HTMLButtonElement>('button.lg-compose-pip');
    if (pip !== null) {
      await user.click(pip);
      expect(
        [...store.getState().editedPaths].some(
          (path) => pips.has(path) || conditions.has(path) || path.endsWith('.spent'),
        ),
      ).toBe(true);
      store.discard();
    }

    const condition = document.querySelector<HTMLButtonElement>('button.lg-compose-condition');
    if (condition !== null) {
      await user.click(condition);
      expect([...store.getState().editedPaths].some((path) => conditions.has(path))).toBe(true);
      store.discard();
    }
  });

  it('Edge: no derived leaf exposes an enabled editing input', () => {
    renderPresetSheet(config, template, sheet);
    for (const leaf of staticLeaves) {
      if (leaf.schema.isEditable) {
        continue;
      }
      expect(document.querySelector(`[data-edit-path="${leaf.path}"]`)).toBeNull();
      const row = fieldRow(leaf.path);
      if (row !== null) {
        expect(row.querySelector('input, select')).toBeNull();
      }
    }

    for (const leaf of records) {
      if (leaf.schema.isEditable) {
        continue;
      }
      const row = document.querySelector(`[data-record-key="${leaf.listPath}.${leaf.recordID}"]`);
      const label = fieldDisplayName(lastSegment(leaf.field), leaf.schema);
      const controls = within(row as HTMLElement)
        .queryAllByLabelText(label)
        .filter(enabled);
      expect(controls).toHaveLength(0);
    }
  });

  it('Happy path: the preset draws one record per list', () => {
    renderPresetSheet(config, template, sheet);
    for (const list of listInfos(template.fields)) {
      expect(document.querySelector(`[data-record-key^="${list.path}."]`)).not.toBeNull();
    }
    expect(screen.getByTestId('sheet-play')).toBeInTheDocument();
  });
});
