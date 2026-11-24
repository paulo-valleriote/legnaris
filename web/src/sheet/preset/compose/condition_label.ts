export interface ConditionLabelParts {
  readonly name: string;
  readonly rule?: string;
}

export function conditionLabelParts(label: string): ConditionLabelParts {
  const separator = label.indexOf(':');
  if (separator <= 0) {
    return { name: label };
  }
  const name = label.slice(0, separator).trim();
  const rule = label.slice(separator + 1).trim();
  if (name === '' || rule === '') {
    return { name: label };
  }
  return { name, rule };
}

export function conditionIsOn(value: unknown): boolean {
  return typeof value === 'number' && Number.isFinite(value) && value !== 0;
}
