import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { valueAtPath } from '../api/models/sheet';
import type { FieldNode, TemplateDetail } from '../api/models/template';
import { parseTemplateDetail } from '../api/models/template';
import { message } from '../l10n/strings';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { configPathErrors } from '../sheet/preset/compose/config_paths';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import { DEMO_TEMPLATE } from '../landing/demo_character';
import { createDemoEditStore } from '../landing/demo_store';
import {
  FIFTH_DEMO_SHEET,
  FIFTH_DEMO_TEMPLATE,
  FIFTH_DEMO_TEMPLATE_ID,
} from '../landing/fifth_demo_character';
import FifthDemoSheet from '../landing/fifth_demo_sheet';
import { templateDerivation } from '../landing/template_derivation';
import goldenCharacter from '../../../testdata/templates/fifth_character_v1.json';

const here = path.dirname(fileURLToPath(import.meta.url));
const srcRoot = path.resolve(here, '..');
const testRoot = path.resolve(here);
const demoModule = path.resolve(srcRoot, 'landing/fifth_demo_sheet.tsx');
const demoDataModule = path.resolve(srcRoot, 'landing/fifth_demo_character.ts');
const MAX_DEMO_BYTES = 30 * 1024;

function sourceFiles(dir: string): readonly string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name);
    return entry.isDirectory() ? sourceFiles(full) : [full];
  });
}

function productionSources(): readonly string[] {
  return sourceFiles(srcRoot).filter(
    (file) =>
      file.startsWith(`${testRoot}${path.sep}`) === false && /\.(ts|tsx|js|jsx)$/.test(file),
  );
}

async function tabToRoll(
  user: ReturnType<typeof userEvent.setup>,
  name: RegExp,
): Promise<HTMLElement> {
  for (let press = 0; press < 200; press += 1) {
    await user.tab();
    const active = document.activeElement;
    if (active instanceof HTMLElement && name.test(active.getAttribute('aria-label') ?? '')) {
      return active;
    }
  }
  throw new Error(`no keyboard stop named ${name}`);
}

function templateSurface(template: TemplateDetail): Record<string, unknown> {
  const surface: Record<string, unknown> = {
    id: template.id,
    name: template.name,
    kinds: template.kinds,
    gameSystem: template.gameSystem,
    dice: template.dice,
    actions: template.actions,
  };
  const walk = (nodes: Readonly<Record<string, FieldNode>>, prefix: string): void => {
    for (const [key, node] of Object.entries(nodes)) {
      const path = prefix === '' ? key : `${prefix}.${key}`;
      if (node.kind === 'leaf') {
        surface[path] = node.schema;
        continue;
      }
      if (node.kind === 'group') {
        surface[`${path}:group`] = { label: node.label, columns: node.columns };
        walk(node.children, path);
        continue;
      }
      surface[`${path}:list`] = {
        label: node.list.label,
        columns: node.list.columns,
        capacity: node.list.capacity,
        dice: node.list.dice,
        notD20: node.list.notD20,
      };
      for (const [field, leaf] of Object.entries(node.list.shape)) {
        surface[`${path}.${field}`] = leaf.schema;
      }
    }
  };
  walk(template.fields, '');
  return surface;
}

describe('the prepared 5E Character demo', () => {
  const fetchSpy = vi.fn(() => Promise.reject(new Error('the demo must stay offline')));

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchSpy);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchSpy.mockClear();
  });

  it('Happy path: it draws the 5E Character preset over the invented sample character', () => {
    render(<FifthDemoSheet />);

    expect(screen.getByRole('heading', { level: 2, name: 'Ysolde Ashgrave' })).toBeInTheDocument();
    expect(screen.getByTestId('landing-fifth-demo-sheet')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-play')).toBeInTheDocument();
    expect(screen.getByTestId('compose-ability-grid')).toBeInTheDocument();
    expect(screen.getByTestId('compose-save-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-skill-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-conditions')).toBeInTheDocument();
    expect(screen.getByTestId('compose-hit-points')).toBeInTheDocument();
    expect(screen.getAllByTestId('compose-record-table')).toHaveLength(5);
    expect(screen.getAllByText('Cleric').length).toBeGreaterThan(0);
    expect(screen.getByText('Life Domain')).toBeInTheDocument();
    expect(screen.getByText('Human')).toBeInTheDocument();
    expect(screen.getByText('Acolyte')).toBeInTheDocument();
    expect(screen.getByTestId('landing-fifth-demo-note')).toHaveTextContent(
      message('landing.fifthDemoNote'),
    );
    expect(screen.getByTestId('landing-fifth-demo-note').textContent).toBe(
      message('landing.fifthDemoNote'),
    );
  });

  it('Happy path: the local 5E template matches the golden DTO for paths, labels, formulas and dice', () => {
    const golden = parseTemplateDetail(goldenCharacter.data);

    expect(templateSurface(FIFTH_DEMO_TEMPLATE)).toEqual(templateSurface(golden));
  });

  it('Happy path: the template formulas recompute the score, its modifier and every dependent value', () => {
    const store = createDemoEditStore(
      FIFTH_DEMO_TEMPLATE,
      FIFTH_DEMO_SHEET,
      templateDerivation(FIFTH_DEMO_TEMPLATE),
    );
    const read = (path: string) => valueAtPath(store.getState().data, path);

    store.setPath('abilities.wisdom.score', 18);

    expect(read('abilities.wisdom.score')).toBe(18);
    expect(read('abilities.wisdom.mod')).toBe(4);
    expect(read('saves.wisdom.total')).toBe(6);
    expect(read('spellcasting.saveDC')).toBe(14);
    expect(read('passive.perception')).toBe(14);
    expect(store.getState().editedPaths).toEqual(new Set(['abilities.wisdom.score']));
  });

  it('Happy path: every bound preset path exists in the local template copy', () => {
    expect(FIFTH_DEMO_TEMPLATE.id).toBe(FIFTH_DEMO_TEMPLATE_ID);
    expect(FIFTH_DEMO_SHEET.templateId).toBe(FIFTH_DEMO_TEMPLATE_ID);
    expect(FIFTH_DEMO_SHEET.name).toBe('Ysolde Ashgrave');
    expect(fifthCharacterConfig.templateId).toBe(FIFTH_DEMO_TEMPLATE_ID);
    expect(configPathErrors(fifthCharacterConfig, FIFTH_DEMO_TEMPLATE)).toEqual([]);
  });

  it('Happy path: a keyboard visitor reaches a roll and returns it to the page', async () => {
    const user = userEvent.setup();
    render(<FifthDemoSheet />);

    const roll = await tabToRoll(user, /^Roll Wisdom modifier/);
    expect(roll).toHaveAttribute('data-rollable', 'true');

    await user.keyboard('{Enter}');

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Happy path: the dice panel announces the result through a polite live region', async () => {
    const user = userEvent.setup();
    render(<FifthDemoSheet />);

    await user.click(screen.getByRole('button', { name: /^Roll Wisdom modifier/ }));

    const live = document.querySelector('.lg-dice-panel [role="status"][aria-live="polite"]');
    expect(live).not.toBeNull();
    expect(live).toHaveAttribute('aria-atomic', 'true');
    expect(live?.textContent).toContain('Wisdom modifier');
  });

  it('Edge: opening the demo makes no network request', () => {
    render(<FifthDemoSheet />);

    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Edge: no production module imports the demo, so it stays out of the initial bundle', () => {
    const importers = productionSources().filter((file) =>
      readFileSync(file, 'utf8').includes('fifth_demo_sheet'),
    );

    expect(importers).toEqual([]);
  });

  it('Edge: the demo stays inside the LGN-345 source-size ceiling and ships no extra asset', () => {
    const sources = [demoModule, demoDataModule].map((file) => readFileSync(file, 'utf8'));
    const gzipped = gzipSync(sources.join('\n'));

    expect(gzipped.byteLength).toBeLessThanOrEqual(MAX_DEMO_BYTES);
    for (const source of sources) {
      expect(source).not.toMatch(/fetch\(/);
      expect(source).not.toMatch(/https?:\/\//);
      expect(source).not.toMatch(/\.(webp|png|jpe?g|svg)/);
    }
  });

  it('Edge: the fantasy demo keeps demo_v1 and its lazy sheet chunk', async () => {
    const demo = readFileSync(path.resolve(here, '../landing/demo_screen.tsx'), 'utf8');

    expect(DEMO_TEMPLATE.id).toBe('demo_v1');
    expect(demo).toContain("lazy(() => import('./demo_sheet'))");
    expect(demo).not.toContain('fifth_demo');
    expect(presetFromConfig(fifthCharacterConfig).templateId).toBe(FIFTH_DEMO_TEMPLATE_ID);
  });

  it('Happy path: the note is the exact prepared-demo sentence', () => {
    expect(message('landing.fifthDemoNote')).toBe(
      'An invented character on the 5E Character template.',
    );
    const rendered = render(<FifthDemoSheet />);
    const note = within(rendered.container).getByTestId('landing-fifth-demo-note');
    expect(note.textContent).toBe('An invented character on the 5E Character template.');
  });

  it('Edge: the fantasy demo still renders its own character', async () => {
    const { default: DemoScreen } = await import('../landing/demo_sheet');
    render(<DemoScreen />);

    expect(screen.getByRole('heading', { level: 2, name: 'Rowan Vale' })).toBeInTheDocument();
    expect(screen.getAllByText(message('sheet.play.damage')).length).toBeGreaterThan(0);
  });
});
