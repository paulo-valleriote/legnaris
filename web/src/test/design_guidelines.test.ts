import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * The guidelines contract: `web/DESIGN.md` is the sole written source of the visual rules, and
 * `web/AGENTS.md` keeps only development, security and architecture rules and defers to it
 * (UIF-33, SC-006).
 *
 * `src/theme/` is the sanctioned literal-value exception, so it is never scanned here either;
 * this suite governs the two documents, not the token source.
 */
const here = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(here, '../..');
const designPath = path.resolve(webRoot, 'DESIGN.md');
const agentsPath = path.resolve(webRoot, 'AGENTS.md');

function read(file: string): string {
  return existsSync(file) ? readFileSync(file, 'utf8') : '';
}

/** The sections the design document must carry (UIF-33). */
const MANDATORY_SECTIONS = [
  'Principles',
  'Token architecture',
  'Colour',
  'Typography',
  'Spacing and layout',
  'Radius, border and elevation',
  'Motion',
  'Component specifications',
  'Accessibility',
  'Security posture',
  'Per-screen patterns',
  'Enforcement',
  'Anti-patterns',
] as const;

function sectionHeadings(markdown: string): string[] {
  return [...markdown.matchAll(/^##\s+(.+?)\s*$/gm)].map((match) => match[1].toLowerCase());
}

/** The mandatory sections a document is missing. */
function missingSections(markdown: string, sections: readonly string[]): string[] {
  const headings = sectionHeadings(markdown);
  return sections.filter(
    (section) =>
      !headings.some(
        (heading) =>
          heading === section.toLowerCase() || heading.startsWith(`${section.toLowerCase()}`),
      ),
  );
}

/** Table rows that restate a token's literal value — a visual rule in the wrong document. */
function restatedTokenRows(markdown: string): string[] {
  return markdown
    .split('\n')
    .filter(
      (line) => /^\s*\|/.test(line) && /--lg-/.test(line) && /(\d+px|#[0-9a-fA-F]{3,8})/.test(line),
    );
}

describe('design guidelines (UIF-33, SC-006)', () => {
  it('Edge: a design document missing a mandatory section is reported', () => {
    expect(missingSections('## Principles\n\nText.\n', MANDATORY_SECTIONS)).toContain('Colour');
  });

  it('Edge: a developer-rules document restating a token value table is reported', () => {
    const restating =
      '| Type scale | `--lg-type-12` (12px) |\n| Radius | `--lg-radius-sm` (4px) |\n';

    expect(restatedTokenRows(restating)).toHaveLength(2);
  });

  it('Happy path: DESIGN.md carries every mandatory section', () => {
    expect(missingSections(read(designPath), MANDATORY_SECTIONS)).toEqual([]);
  });

  it('Happy path: AGENTS.md points to DESIGN.md', () => {
    expect(read(agentsPath)).toContain('DESIGN.md');
  });

  it('Happy path: the shared layer is sanctioned in DESIGN.md and AGENTS.md (UIP-03)', () => {
    const design = read(designPath);
    const agents = read(agentsPath);
    expect(design).toContain('src/ui/');
    expect(agents).toContain('src/ui/');
    expect(design).toContain('lg-<block>');
  });

  it('Edge: the developer-rules document restates no visual token value', () => {
    const agents = read(agentsPath);

    expect(agents.length).toBeGreaterThan(0);
    expect(restatedTokenRows(agents)).toEqual([]);
  });
});
