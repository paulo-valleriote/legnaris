export const GAME_SYSTEM_CATALOG_ENTRY = {
  id: 'fifth_character_v1',
  name: '5E Character',
  default: false,
  kinds: ['character'],
  fields: {},
  game_system: {
    id: 'fifth',
    name: '5E',
    compatibility_label: '5E compatible',
    notice: { label: 'SRD 5.2.1', path: '/srd' },
  },
};

export const GAME_SYSTEM_TEMPLATE_DETAIL = GAME_SYSTEM_CATALOG_ENTRY;

export const GAME_SYSTEM_CATALOG = [GAME_SYSTEM_CATALOG_ENTRY];

export const GAME_SYSTEM_ENTRY_NAME = /5E Character/;
