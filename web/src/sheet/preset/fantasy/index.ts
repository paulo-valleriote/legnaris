import type { SheetPreset } from '../types';
import { FantasyAbilities } from './abilities';
import { FantasyHero } from './hero';

export const fantasyPreset: SheetPreset = {
  templateId: 'fantasy_v1',
  Header: FantasyHero,
  sections: [{ key: 'stats', Component: FantasyAbilities }],
};
