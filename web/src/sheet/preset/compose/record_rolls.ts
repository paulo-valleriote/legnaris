import { parseDice, type Expression } from '../../../dice/parse';

export function formatAttackModifier(value: number): string {
  return value >= 0 ? `+${value}` : `${value}`;
}

export function parseDamageExpression(text: string): Expression | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return undefined;
  }
  const parsed = parseDice(trimmed);
  if (!parsed.ok) {
    return undefined;
  }
  const rollsDice = parsed.value.terms.some((term) => term.kind === 'dice');
  const readsField = parsed.value.terms.some((term) => term.kind === 'field');
  return rollsDice && !readsField ? parsed.value : undefined;
}
