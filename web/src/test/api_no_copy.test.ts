import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import * as ts from 'typescript';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const apiDir = path.resolve(here, '../api');

const USER_FACING_PROPERTIES = new Set(['message', 'userMessage', 'displayMessage']);

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = path.join(dir, entry);

    if (statSync(full).isDirectory()) {
      return entry === 'fixtures' ? [] : sourceFiles(full);
    }

    return entry.endsWith('.ts') ? [full] : [];
  });
}

function literalText(node: ts.Expression | undefined): string | undefined {
  if (node === undefined) {
    return undefined;
  }

  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
    return node.text.trim() === '' ? undefined : node.text;
  }

  if (ts.isTemplateExpression(node)) {
    const staticText = [node.head.text, ...node.templateSpans.map((span) => span.literal.text)]
      .join('')
      .trim();

    return staticText === '' ? undefined : staticText;
  }

  return undefined;
}

function propertyName(name: ts.PropertyName): string | undefined {
  if (
    ts.isIdentifier(name) ||
    ts.isStringLiteral(name) ||
    ts.isNoSubstitutionTemplateLiteral(name)
  ) {
    return name.text;
  }

  return undefined;
}

function copyViolations(fileName: string, source: string): string[] {
  const sourceFile = ts.createSourceFile(
    fileName,
    source,
    ts.ScriptTarget.Latest,
    true,
    fileName.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const violations: string[] = [];

  const report = (node: ts.Node, detail: string) => {
    const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
    violations.push(`${fileName}:${line} ${detail}`);
  };

  const visit = (node: ts.Node): void => {
    if (
      ts.isNewExpression(node) &&
      ts.isIdentifier(node.expression) &&
      node.expression.text === 'Error'
    ) {
      const text = literalText(node.arguments?.[0]);

      if (text !== undefined) {
        report(node, `new Error(${JSON.stringify(text)})`);
      }
    }

    if (ts.isThrowStatement(node)) {
      const text = literalText(node.expression);

      if (text !== undefined) {
        report(node, `throw ${JSON.stringify(text)}`);
      }
    }

    if (
      ts.isPropertyAssignment(node) &&
      USER_FACING_PROPERTIES.has(propertyName(node.name) ?? '')
    ) {
      const text = literalText(node.initializer);

      if (text !== undefined) {
        report(node, `${propertyName(node.name) ?? ''}: ${JSON.stringify(text)}`);
      }
    }

    if (ts.isReturnStatement(node)) {
      const text = literalText(node.expression);

      if (text !== undefined && /\s/.test(text.trim())) {
        report(node, `return ${JSON.stringify(text)}`);
      }
    }

    ts.forEachChild(node, visit);
  };

  visit(sourceFile);

  return violations;
}

describe('user-facing copy in the API module (LGN-293)', () => {
  it('Edge: no source file carries a literal user-facing message', () => {
    const files = sourceFiles(apiDir);
    expect(files.length).toBeGreaterThan(0);

    const violations = files.flatMap((file) => copyViolations(file, readFileSync(file, 'utf8')));

    expect(violations).toEqual([]);
  });

  it('Edge: an Error built from a literal is refused', () => {
    const source = "export function fail() {\n  throw new Error('Something broke');\n}\n";

    expect(copyViolations('probe.ts', source)).toHaveLength(1);
  });

  it('Edge: an Error built from a template with static copy is refused', () => {
    const source =
      'export function fail(code: string) {\n  throw new Error(`Request failed: ${code}`);\n}\n';

    expect(copyViolations('probe.ts', source)).toHaveLength(1);
  });

  it('Edge: a thrown literal is refused', () => {
    const source = "export function fail() {\n  throw 'Something broke';\n}\n";

    expect(copyViolations('probe.ts', source)).toHaveLength(1);
  });

  it('Edge: a literal message property is refused', () => {
    const source = "export const failure = { message: 'Something broke' };\n";

    expect(copyViolations('probe.ts', source)).toHaveLength(1);
  });

  it('Edge: a sentence-like returned literal is refused', () => {
    const source = "export function copy() {\n  return 'Something broke';\n}\n";

    expect(copyViolations('probe.ts', source)).toHaveLength(1);
  });

  it('Happy path: an Error whose message resolves through src/l10n passes', () => {
    const source = [
      "import { message } from '../l10n/strings';",
      'export function fail() {',
      "  throw new Error(message('api.error.unknown'));",
      '}',
      '',
    ].join('\n');

    expect(copyViolations('probe.ts', source)).toHaveLength(0);
  });

  it('Happy path: a returned failure code and a diagnostic reason pass', () => {
    const source = [
      'export function kind(): string {',
      "  return 'generationFailed';",
      '}',
      'export function failure(reason: string): Error {',
      '  return new Error(reason);',
      '}',
      '',
    ].join('\n');

    expect(copyViolations('probe.ts', source)).toHaveLength(0);
  });

  it('Happy path: a diagnostic reason on a typed failure passes', () => {
    const source = [
      'export function fail() {',
      "  throw new DefensiveParseFailure({ reason: 'response body is empty' });",
      '}',
      '',
    ].join('\n');

    expect(copyViolations('probe.ts', source)).toHaveLength(0);
  });
});
