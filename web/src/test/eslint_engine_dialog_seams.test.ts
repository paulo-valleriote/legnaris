import { beforeAll, describe, expect, it } from 'vitest';
import {
  DIALOG_MESSAGE,
  hasMessage,
  messagesAt,
  restricted,
  ruleIds,
  warmUpEslint,
} from './eslint_harness';

beforeAll(warmUpEslint, 60000);

describe('ESLint drag dependency seam (LGN-292)', () => {
  it('Error: the engine may not import @dnd-kit', async () => {
    const messages = await messagesAt(
      'src/engine/probe.ts',
      "import { DndContext } from '@dnd-kit/core';\nexport const drag = DndContext;",
    );

    expect(ruleIds(messages)).toContain('no-restricted-imports');
  });

  it('Happy path: Arrange frames outside the engine may import @dnd-kit', async () => {
    const code = "import { DndContext } from '@dnd-kit/core';\nexport const drag = DndContext;";

    expect(restricted(await messagesAt('src/sheet/arrange_frames.tsx', code))).toEqual([]);
  });
});

describe('ESLint accessible dialog (LGN-292)', () => {
  it('Error: a native dialog element is refused', async () => {
    const messages = await messagesAt(
      'src/sheet/probe.tsx',
      'export const element = <dialog open />;',
    );

    expect(ruleIds(messages)).toContain('no-restricted-syntax');
    expect(hasMessage(messages, DIALOG_MESSAGE)).toBe(true);
  });

  it('Happy path: the accessible overlay with role and aria-modal is allowed', async () => {
    const code = 'export const element = <div role="dialog" aria-modal="true" />;';

    expect(restricted(await messagesAt('src/ui/dialog.tsx', code))).toEqual([]);
  });
});
