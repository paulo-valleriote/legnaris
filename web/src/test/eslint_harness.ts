import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESLint } from 'eslint';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const eslint = new ESLint({ cwd: webRoot });

export const STORAGE_MESSAGE =
  'Credentials are memory-only; storage is allowed only in the theme/view-state seams';

export const DIALOG_MESSAGE =
  'Use the accessible overlay (role="dialog", aria-modal) instead of a native <dialog>.';

export interface LintMessage {
  readonly ruleId: string | null;
  readonly message: string;
}

export async function messagesAt(file: string, code: string): Promise<LintMessage[]> {
  const [result] = await eslint.lintText(code, { filePath: path.join(webRoot, file) });
  return result.messages.map(({ ruleId, message }) => ({ ruleId, message }));
}

export function restricted(messages: readonly LintMessage[]): LintMessage[] {
  return messages.filter(({ ruleId }) => ruleId?.startsWith('no-restricted'));
}

export function ruleIds(messages: readonly LintMessage[]): string[] {
  return messages.flatMap(({ ruleId }) => (ruleId === null ? [] : [ruleId]));
}

export function hasMessage(messages: readonly LintMessage[], expected: string): boolean {
  return messages.some(({ message }) => message.includes(expected));
}

export async function warmUpEslint(): Promise<void> {
  await eslint.lintText('export {};', { filePath: path.join(webRoot, 'src/sheet/warmup.ts') });
}
