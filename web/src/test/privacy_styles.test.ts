import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const privacyCss = readFileSync(path.resolve(here, '../legal/privacy.css'), 'utf8');
const dialogSource = readFileSync(path.resolve(here, '../legal/privacy_dialog.tsx'), 'utf8');
const screenSource = readFileSync(path.resolve(here, '../legal/privacy_screen.tsx'), 'utf8');

function blockFor(selector: string): string {
  const start = privacyCss.indexOf(selector);
  expect(start, `${selector} is declared`).toBeGreaterThanOrEqual(0);
  return privacyCss.slice(start, privacyCss.indexOf('}', start));
}

describe('privacy surface styles (DESIGN.md, Privacy)', () => {
  it('Happy path: body links read the link role, never the accent fill', () => {
    const link = blockFor('.lg-privacy__body a');

    expect(link).toContain('var(--lg-color-link)');
    expect(link).toContain('var(--lg-underline-offset)');
    expect(privacyCss).not.toContain('var(--lg-color-accent)');
  });

  it('Edge: every body link has a hover tone', () => {
    expect(blockFor('.lg-privacy__body a:hover')).toContain('var(--lg-color-link-hover)');
  });

  it('Happy path: the page keeps the readable measure inside the shell gutter', () => {
    expect(privacyCss).toContain('max-width: var(--lg-size-content-max)');
    expect(privacyCss).toContain('max-width: var(--lg-size-narrow-max)');
    expect(privacyCss).toContain('var(--lg-space-24)');
    expect(privacyCss).not.toMatch(/\b\d+px\b/);
  });

  it('Happy path: the document dialog is the shared dialog, not a hand-rolled overlay', () => {
    expect(dialogSource).toContain("from '../ui/dialog'");
    expect(dialogSource).not.toContain('catalog.css');
    expect(dialogSource).not.toContain('lg-overlay');
    expect(dialogSource).not.toContain('<svg');
  });

  it('Edge: the dialog keeps the one scroll body and the shared close label', () => {
    expect(dialogSource).toContain('privacy-dialog-body');
    expect(dialogSource).toContain("message('privacy.close')");
    expect(privacyCss).toContain('.lg-privacy-dialog__scroll');
    expect(privacyCss).toContain('overscroll-behavior: contain');
  });

  it('Happy path: the public page opens on the page header with a way back', () => {
    expect(screenSource).toContain('PageHeader');
    expect(screenSource).toContain('backTo="/"');
    expect(dialogSource).toContain('policyDocument');
  });
});
