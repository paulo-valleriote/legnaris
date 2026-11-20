import { messageWith } from '../l10n/strings';

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

export function rollName(label: string): RegExp {
  return new RegExp(`^${escapeRegExp(messageWith('dice.roll', { label }))}(, .+)?$`);
}

export function initiativeRollName(value: string): string {
  return messageWith('sheet.play.initiativeRoll', { value });
}
