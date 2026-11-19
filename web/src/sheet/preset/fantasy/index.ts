import type { SheetPreset } from '../types';
import { FantasyHero } from './hero';
import { FantasyTableRules } from './rules';
import { FantasySpells } from './spells';

export const fantasyPreset: SheetPreset = {
  templateId: 'fantasy_v1',
  Header: FantasyHero,
  sections: [
    { key: 'spells', Component: FantasySpells },
    { key: 'rules', Component: FantasyTableRules },
  ],
};
