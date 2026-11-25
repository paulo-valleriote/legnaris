import type { ReactNode } from 'react';
import { usePresetContext } from '../preset_context';

export function GroupTitle({ children }: { readonly children: ReactNode }) {
  const { groupHeadingLevel } = usePresetContext();
  const Heading = groupHeadingLevel === 3 ? 'h3' : 'h2';
  return <Heading className="lg-compose-group__title">{children}</Heading>;
}
