import type { SheetPreset } from '../types';
import { FantasyAbilities } from './abilities';
import { FantasyAttacks } from './attacks';
import { CombatStrip } from './combat_strip';
import { FantasyHero } from './hero';
import { FantasyTableRules } from './rules';
import { FantasySaves, FantasySkills } from './saves_skills';
import { FantasySpells } from './spells';
import { FantasyStatBlock, StatBlockHeader } from './stat_block';
import { isStatBlockKind } from './stat_block_data';

export const fantasyPreset: SheetPreset = {
  templateId: 'fantasy_v1',
  Header: FantasyHero,
  statBlock: { when: isStatBlockKind, Header: StatBlockHeader, Body: FantasyStatBlock },
  sections: [
    { key: 'combat', Component: CombatStrip },
    { key: 'attacks', Component: FantasyAttacks },
    { key: 'stats', Component: FantasyAbilities },
    { key: 'saves', Component: FantasySaves },
    { key: 'skills', Component: FantasySkills },
    { key: 'spells', Component: FantasySpells },
    { key: 'rules', Component: FantasyTableRules },
  ],
};
