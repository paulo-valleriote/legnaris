import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESLint, type Linter } from 'eslint';
import { beforeAll, describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(here, '../..');

const LITERAL_RULE = 'react/jsx-no-literals';

const eslint = new ESLint({ cwd: webRoot });

async function literalFindings(
  relativePath: string,
  source: string,
): Promise<Linter.LintMessage[]> {
  const [result] = await eslint.lintText(source, {
    filePath: path.join(webRoot, relativePath),
  });

  return result.messages.filter((finding) => finding.ruleId === LITERAL_RULE);
}

function component(child: string): string {
  return `export function Probe() {\n  return <div>${child}</div>;\n}\n`;
}

describe('UI copy guard (LGN-293)', () => {
  beforeAll(async () => {
    await literalFindings('src/sheet/warmup.tsx', component('{null}'));
  }, 120_000);

  it('Edge: a literal text child of a component fails the rule', async () => {
    const findings = await literalFindings('src/sheet/probe.tsx', component('<p>Saved</p>'));

    expect(findings).toHaveLength(1);
    expect(findings[0]?.message).toContain('Saved');
  });

  it('Edge: a literal in a JSX expression container fails the rule', async () => {
    const findings = await literalFindings('src/sheet/probe.tsx', component("{'Saved'}"));

    expect(findings).toHaveLength(1);
    expect(findings[0]?.message).toContain('Saved');
  });

  it('Edge: a literal inside a nested component fails the rule', async () => {
    const findings = await literalFindings(
      'src/engine/probe.tsx',
      'export function Probe() {\n  return <section><h2>Attributes</h2></section>;\n}\n',
    );

    expect(findings).toHaveLength(1);
    expect(findings[0]?.message).toContain('Attributes');
  });

  it('Happy path: copy resolved through src/l10n passes', async () => {
    const source = [
      "import { message } from '../l10n/strings';",
      'export function Probe() {',
      "  return <p>{message('common.retry')}</p>;",
      '}',
      '',
    ].join('\n');

    expect(await literalFindings('src/sheet/probe.tsx', source)).toHaveLength(0);
  });

  it('Edge: allow-listed punctuation is not treated as copy', async () => {
    const punctuation = ['·', '×', '—', '–', '/', ':', '+', '-', '%', '…'];

    const source = [
      'export function Probe({ a, b }: { a: string; b: string }) {',
      '  return (',
      '    <p>',
      '      {a}',
      ...punctuation.flatMap((token) => [`      ${token}`, '      {b}']),
      '    </p>',
      '  );',
      '}',
      '',
    ].join('\n');

    expect(await literalFindings('src/campaign/probe.tsx', source)).toHaveLength(0);
  });

  it('Edge: attribute strings are not JSX children and stay out of the rule', async () => {
    const source = [
      'export function Probe({ value }: { value: string }) {',
      '  return <p className="lg-probe" aria-label="Saved">{value}</p>;',
      '}',
      '',
    ].join('\n');

    expect(await literalFindings('src/sheet/probe.tsx', source)).toHaveLength(0);
  });

  it('Edge: the test tree is exempt from the rule', async () => {
    const source = "import { render } from '@testing-library/react';\nrender(<p>Saved</p>);\n";

    expect(await literalFindings('src/test/probe.test.tsx', source)).toHaveLength(0);
  });
});
