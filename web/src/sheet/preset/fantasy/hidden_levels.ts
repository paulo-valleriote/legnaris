export function formatLevelRanges(levels: readonly number[]): string {
  const ordered = [...new Set(levels)].sort((left, right) => left - right);
  const ranges: string[] = [];
  let start: number | undefined;
  let end: number | undefined;

  for (const level of ordered) {
    if (start === undefined || end === undefined) {
      start = level;
      end = level;
      continue;
    }
    if (level === end + 1) {
      end = level;
      continue;
    }
    ranges.push(start === end ? String(start) : `${start}–${end}`);
    start = level;
    end = level;
  }

  if (start !== undefined && end !== undefined) {
    ranges.push(start === end ? String(start) : `${start}–${end}`);
  }

  return ranges.join(', ');
}
