const CLASSES = {
  Barbarian: 'Path of the Berserker',
  Bard: 'College of Lore',
  Cleric: 'Life Domain',
  Druid: 'Circle of the Land',
  Fighter: 'Champion',
  Monk: 'Warrior of the Open Hand',
  Paladin: 'Oath of Devotion',
  Ranger: 'Hunter',
  Rogue: 'Thief',
  Sorcerer: 'Draconic Sorcery',
  Warlock: 'Fiend Patron',
  Wizard: 'Evoker',
};

const SPECIES = new Set(['Dragonborn', 'Dwarf', 'Elf', 'Gnome', 'Goliath', 'Halfling', 'Human', 'Orc', 'Tiefling']);
const BACKGROUNDS = new Set(['Acolyte', 'Criminal', 'Sage', 'Soldier']);
const ABILITIES = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'];

function isObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

module.exports = (output) => {
  let sheet;
  try {
    sheet = JSON.parse(output);
  } catch (err) {
    return { pass: false, score: 0, reason: 'sheet is not valid JSON' };
  }
  if (!isObject(sheet)) {
    return { pass: false, score: 0, reason: 'sheet is not an object' };
  }

  const problems = [];
  const identity = sheet.identity;
  if (!isObject(identity)) {
    return { pass: false, score: 0, reason: 'identity is missing' };
  }
  if (typeof identity.name !== 'string' || identity.name.trim().length < 2) {
    problems.push('name is empty');
  }
  if (!Object.prototype.hasOwnProperty.call(CLASSES, identity.class)) {
    problems.push(`class ${JSON.stringify(identity.class)} is not an SRD class`);
  }
  if (!SPECIES.has(identity.species)) {
    problems.push(`species ${JSON.stringify(identity.species)} is not an SRD species`);
  }
  if (!BACKGROUNDS.has(identity.background)) {
    problems.push(`background ${JSON.stringify(identity.background)} is not an SRD background`);
  }

  const level = identity.level;
  if (!Number.isInteger(level) || level < 1 || level > 20) {
    problems.push(`level ${JSON.stringify(level)} is outside 1..20`);
  } else {
    const expectedBonus = 2 + Math.floor((level - 1) / 4);
    if (identity.proficiencyBonus !== expectedBonus) {
      problems.push(`proficiency bonus ${identity.proficiencyBonus} should be ${expectedBonus} at level ${level}`);
    }
    const expectedSubclass = level >= 3 ? CLASSES[identity.class] : '';
    if (expectedSubclass !== undefined && identity.subclass !== expectedSubclass) {
      problems.push(`subclass ${JSON.stringify(identity.subclass)} should be ${JSON.stringify(expectedSubclass)}`);
    }
  }

  const abilities = sheet.abilities;
  if (!isObject(abilities)) {
    problems.push('abilities are missing');
  } else {
    for (const ability of ABILITIES) {
      const group = abilities[ability];
      if (!isObject(group)) {
        problems.push(`${ability} is missing`);
        continue;
      }
      const score = group.score;
      if (!Number.isInteger(score) || score < 1 || score > 30) {
        problems.push(`${ability} score ${JSON.stringify(score)} is outside 1..30`);
        continue;
      }
      const expectedMod = Math.floor((score - 10) / 2);
      if (group.mod !== expectedMod) {
        problems.push(`${ability} modifier ${group.mod} should be ${expectedMod}`);
      }
    }
  }

  const hp = isObject(sheet.combat) ? sheet.combat.hp : undefined;
  if (!isObject(hp) || !Number.isInteger(hp.max) || hp.max < 1) {
    problems.push('combat.hp.max is not a positive integer');
  } else if (!Number.isInteger(hp.current) || hp.current < 0 || hp.current > hp.max) {
    problems.push('combat.hp.current is outside 0..max');
  }

  for (const [name, records] of [
    ['features', isObject(sheet.features) ? sheet.features.list : undefined],
    ['attacks', isObject(sheet.attacks) ? sheet.attacks.list : undefined],
    ['spells', isObject(sheet.spells) ? sheet.spells.list : undefined],
    ['resources', isObject(sheet.resources) ? sheet.resources.list : undefined],
    ['inventory', isObject(sheet.inventory) ? sheet.inventory.items : undefined],
  ]) {
    if (!isObject(records)) {
      problems.push(`${name} list is missing`);
    }
  }

  const features = isObject(sheet.features) && isObject(sheet.features.list) ? Object.values(sheet.features.list) : [];
  for (const [index, feature] of features.entries()) {
    if (!isObject(feature) || typeof feature.text !== 'string') {
      problems.push(`feature ${index} has no text`);
      continue;
    }
    if (Array.from(feature.text).length > 300) {
      problems.push(`feature ${index} text is over 300 characters`);
    }
  }

  if (problems.length > 0) {
    return { pass: false, score: 0, reason: problems.join('; ') };
  }
  return { pass: true, score: 1, reason: 'sheet is coherent' };
};
