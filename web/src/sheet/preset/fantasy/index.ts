import type { SheetPreset } from '../types';
import { FantasyAbilities } from './abilities';
import { CombatStrip } from './combat_strip';
import { FantasyHero } from './hero';

export const fantasyPreset: SheetPreset = {
  templateId: 'fantasy_v1',
  Header: FantasyHero,
  sections: [
    { key: 'combat', Component: CombatStrip },
    { key: 'stats', Component: FantasyAbilities },
  ],
};
