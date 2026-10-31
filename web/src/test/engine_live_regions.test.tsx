import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { FieldSchema } from '../api/models/template';
import { parseTemplateDetail } from '../api/models/template';
import { SheetFieldControl } from '../engine/field_control';
import { SheetForm } from '../engine/sheet_form';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';

const here = path.dirname(fileURLToPath(import.meta.url));
const LIVE_SELECTOR = '[aria-live], [role="status"], [role="alert"], [role="log"], output';
const BASE_URL = 'http://api.test';

function capture(name: string): Record<string, unknown> {
  const file = path.join(here, '..', 'api', 'fixtures', `${name}.json`);
  const parsed = JSON.parse(readFileSync(file, 'utf8')) as {
    response: { body: { data: Record<string, unknown> } };
  };
  return parsed.response.body.data;
}

function derivedSchema(overrides: Partial<FieldSchema> = {}): FieldSchema {
  return {
    type: 'number',
    label: 'Total',
    isEditable: false,
    defaultValue: 0,
    formula: 'str + 1',
    ...overrides,
  };
}

function liveRegionsIn(root: ParentNode): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(LIVE_SELECTOR));
}

describe('derived value live regions', () => {
  const values: ReadonlyArray<readonly [string, unknown]> = [
    ['Edge: a null value', null],
    ['Edge: an undefined value', undefined],
    ['Edge: an empty string', ''],
    ['Edge: zero', 0],
    ['Edge: a negative number', -5],
    ['Edge: a very long string', 'x'.repeat(400)],
    ['Happy path: a positive number', 12],
  ];

  it.each(values)('%s renders a named non-live readout', (_name, value) => {
    const { container } = render(
      <SheetFieldControl path="derived.total" schema={derivedSchema()} value={value} />,
    );

    const readout = screen.getByRole('group', { name: 'Total' });
    expect(readout.tagName).toBe('SPAN');
    expect(readout).toHaveClass('lg-field__readonly--derived');
    expect(liveRegionsIn(container)).toHaveLength(0);
    expect(readout).not.toHaveAttribute('role', 'status');
    expect(readout).not.toHaveAttribute('aria-live');
    expect(readout).not.toHaveAttribute('tabindex');
    expect(readout).toHaveAccessibleDescription('Calculated');
  });

  it('Edge: a long value is carried in full inside the named readout', () => {
    const long = 'y'.repeat(400);
    render(<SheetFieldControl path="derived.total" schema={derivedSchema()} value={long} />);

    expect(screen.getByRole('group', { name: 'Total' })).toHaveTextContent(long);
  });

  it('Edge: a changing derived value stays silent and keeps one element', () => {
    const { container, rerender } = render(
      <SheetFieldControl path="derived.total" schema={derivedSchema()} value={1} />,
    );
    const before = screen.getByRole('group', { name: 'Total' });

    rerender(<SheetFieldControl path="derived.total" schema={derivedSchema()} value={2} />);

    expect(screen.getByRole('group', { name: 'Total' })).toBe(before);
    expect(before).toHaveTextContent('2');
    expect(liveRegionsIn(container)).toHaveLength(0);
  });

  it('Edge: a derived value with the same label as another does not resolve to an editable control', () => {
    render(
      <>
        <SheetFieldControl
          path="stats.base"
          schema={derivedSchema({ label: 'Base', isEditable: true, formula: undefined })}
          value={3}
          onChanged={() => undefined}
        />
        <SheetFieldControl path="derived.total" schema={derivedSchema()} value={4} />
      </>,
    );

    expect(screen.getByRole('textbox', { name: 'Base' })).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'Base' })).not.toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Total' })).toBeInTheDocument();
  });

  it('Edge: a marked derived value is non-live and drops the Calculated caption', () => {
    const { container } = render(
      <SheetFieldControl
        path="derived.total"
        schema={derivedSchema()}
        value={9}
        mark="recalculated"
      />,
    );

    const readout = screen.getByRole('group', { name: 'Total' });
    expect(readout).toHaveClass('lg-field--marked');
    expect(screen.queryByText('Calculated')).not.toBeInTheDocument();
    expect(liveRegionsIn(container)).toHaveLength(0);
  });

  it('Edge: an editable leaf of an unrecognized type is a non-live readout too', () => {
    const { container } = render(
      <SheetFieldControl
        path="misc.mystery"
        schema={derivedSchema({ type: 'fancy', label: 'Mystery', isEditable: true })}
        value={7}
      />,
    );

    expect(screen.getByRole('group', { name: 'Mystery' })).toHaveTextContent('7');
    expect(liveRegionsIn(container)).toHaveLength(0);
  });

  it('Edge: Tab skips derived readouts and never lands inside them', async () => {
    const user = userEvent.setup();
    render(
      <>
        <SheetFieldControl
          path="stats.base"
          schema={derivedSchema({ label: 'Base', isEditable: true, formula: undefined })}
          value={3}
          onChanged={() => undefined}
        />
        <SheetFieldControl path="derived.total" schema={derivedSchema()} value={4} />
        <SheetFieldControl
          path="stats.bonus"
          schema={derivedSchema({ label: 'Bonus', isEditable: true, formula: undefined })}
          value={1}
          onChanged={() => undefined}
        />
      </>,
    );

    await user.tab();
    expect(screen.getByRole('textbox', { name: 'Base' })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('textbox', { name: 'Bonus' })).toHaveFocus();
  });
});

describe('full fantasy_v1 render', () => {
  const template = parseTemplateDetail(capture('template_detail'));

  it('Happy path: SheetForm in data mode holds no live region and every lone derived readout is named by its row label', () => {
    const { container } = render(<SheetForm fields={template.fields} data={{}} />);

    expect(liveRegionsIn(container)).toHaveLength(0);
    const derived = Array.from(
      container.querySelectorAll<HTMLElement>('.lg-field__readonly--derived'),
    );
    expect(derived.length).toBeGreaterThan(10);
    let namedByRowLabel = 0;
    for (const readout of derived) {
      expect(readout).toHaveAttribute('role', 'group');
      const row = readout.closest<HTMLElement>('[data-row-kind="normal"]');
      const rowLabel = row?.querySelector('.lg-field-row__label-text');
      if (rowLabel === null || rowLabel === undefined) {
        expect(readout.getAttribute('aria-label') ?? '').not.toBe('');
        expect(readout).not.toHaveAttribute('aria-labelledby');
        continue;
      }
      expect(readout).not.toHaveAttribute('aria-label');
      expect(readout.getAttribute('aria-labelledby')).toBe(rowLabel.id);
      expect(readout).toHaveAccessibleName(rowLabel.textContent ?? '');
      namedByRowLabel += 1;
    }
    expect(namedByRowLabel).toBeGreaterThan(0);
  });

  it('Edge: a row mark is plain text read once, outside any live region and outside the readout', () => {
    const { container } = render(
      <SheetForm
        fields={template.fields}
        data={{}}
        rowMarkBuilder={(paths) =>
          paths.includes('stats.strength') ? (
            <span className="lg-field-row__mark" data-mark="recalculated">
              Recalculated
            </span>
          ) : null
        }
      />,
    );

    const marks = screen.getAllByText('Recalculated');
    expect(marks).toHaveLength(1);
    for (const mark of marks) {
      expect(mark.closest(LIVE_SELECTOR)).toBeNull();
      expect(mark.closest('.lg-field__readonly')).toBeNull();
    }
    expect(liveRegionsIn(container)).toHaveLength(0);
  });

  it('Happy path: the mounted sheet page has only the footer summary and the pin announcer as live regions', async () => {
    const sheet = capture('sheet_get');
    const fake = createFakeTransport(
      successOutcome(sheet),
      successOutcome(capture('template_detail')),
    );
    const { container } = renderApp(`/sheets/${String(sheet['id'])}`, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });

    await waitFor(() =>
      expect(container.querySelectorAll('.lg-field__readonly--derived').length).toBeGreaterThan(10),
    );

    const regions = liveRegionsIn(document.body);
    const footer = regions.filter((region) => region.closest('.lg-sheet__footer') !== null);
    const pinAnnouncer = screen.getByTestId('arrange-live');
    expect(regions).toHaveLength(2);
    expect(footer).toHaveLength(1);
    expect(footer[0]).toHaveAttribute('aria-live', 'polite');
    expect(regions).toContain(pinAnnouncer);
    for (const region of regions) {
      expect(region).toBeEmptyDOMElement();
      expect(region.closest('.lg-field-row, .lg-field-table__row')).toBeNull();
    }
    expect(document.querySelectorAll('.lg-field__readonly[role="status"]')).toHaveLength(0);
  });
});
