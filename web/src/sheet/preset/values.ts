export function textValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

export function numberValue(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}
