import vectorsJson from '../../../testdata/dice/vectors.json';

export interface DiceVectorPart {
  readonly sign: 1 | -1;
  readonly label: string;
  readonly value: number;
  readonly rolls?: readonly number[];
}

export interface DiceVectors {
  readonly version: number;
  readonly limits: {
    readonly maxLength: number;
    readonly maxTerms: number;
    readonly minDice: number;
    readonly maxDice: number;
    readonly minFaces: number;
    readonly maxFaces: number;
    readonly maxWholeNumber: number;
  };
  readonly fields: Readonly<Record<string, 'number' | 'text' | 'bool'>>;
  readonly valid: readonly { name: string; expr: string; canonical: string }[];
  readonly invalid: readonly { name: string; expr: string; class: string }[];
  readonly fit: readonly {
    name: string;
    expr: string;
    row: string;
    fits: boolean;
    class?: string;
  }[];
  readonly sections: readonly {
    name: string;
    expr: string;
    rows: readonly string[];
    fits: boolean;
    class?: string;
  }[];
  readonly evaluate: readonly {
    name: string;
    expr: string;
    row: string;
    values: Readonly<Record<string, number>>;
    dice: readonly number[];
    total?: number;
    parts?: readonly DiceVectorPart[];
    error?: string;
  }[];
}

/** The vectors shared with the Go package (`testdata/dice/vectors.json`); both sides must pass every one. */
export const diceVectors = vectorsJson as unknown as DiceVectors;

/**
 * Turns the flat `values` of a vector (dotted path → number) into the nested tree
 * the sheet holds on screen.
 */
export function nestValues(flat: Readonly<Record<string, number>>): Record<string, unknown> {
  const tree: Record<string, unknown> = {};
  for (const [path, value] of Object.entries(flat)) {
    const segments = path.split('.');
    let cursor = tree;
    for (const segment of segments.slice(0, -1)) {
      const next = cursor[segment];
      if (typeof next === 'object' && next !== null) {
        cursor = next as Record<string, unknown>;
      } else {
        const created: Record<string, unknown> = {};
        cursor[segment] = created;
        cursor = created;
      }
    }
    cursor[segments[segments.length - 1]] = value;
  }
  return tree;
}
