import { messageWith } from '../l10n/strings';

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Matches the accessible name of a Row's Roll button, which is `Roll {label}` followed by the
 * dice it throws when the expression has any (`Roll One, d20`).
 */
export function rollName(label: string): RegExp {
  return new RegExp(`^${escapeRegExp(messageWith('dice.roll', { label }))}(, .+)?$`);
}
