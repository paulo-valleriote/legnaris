import type { SheetPreset } from '../types';
import { FantasyAbilities } from './abilities';
import { FantasyAttacks } from './attacks';
import { CombatStrip } from './combat_strip';
import { FantasyHero } from './hero';
import { FantasyTableRules } from './rules';
import { FantasySaves, FantasySkills } from './saves_skills';
import { FantasySpells } from './spells';

export const fantasyPreset: SheetPreset = {
  templateId: 'fantasy_v1',
  Header: FantasyHero,
  sections: [
    { key: 'combat', Component: CombatStrip },
    { key: 'attacks', Component: FantasyAttacks },
    { key: 'stats', Component: FantasyAbilities },
    { key: 'saves', Component: FantasySaves },
    { key: 'skills', Component: FantasySkills },
    { key: 'spells', Component: FantasySpells },
    { key: 'rules', Component: FantasyTableRules },
  ],
  layout: [
    { sections: ['combat'] },
    { columns: [['stats'], ['saves', 'skills'], ['attacks', 'spells']] },
    { sections: ['rules'] },
  ],
};
