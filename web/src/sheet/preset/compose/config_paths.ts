import { buildSections, rowKey } from '../../../engine/field_grouping';
import type { TemplateDetail } from '../../../api/models/template';
import type { PresetConfig, PresetPiece } from './types';

interface KnownTemplate {
  readonly paths: ReadonlySet<string>;
  readonly rows: ReadonlySet<string>;
  readonly sections: ReadonlySet<string>;
}

function knownTemplate(template: TemplateDetail): KnownTemplate {
  const paths = new Set<string>();
  const rows = new Set<string>();
  const sections = new Set<string>();
  for (const section of buildSections(template.fields)) {
    sections.add(section.key);
    for (const row of section.rows) {
      const key = rowKey(row);
      rows.add(key);
      paths.add(key);
    }
    for (const leaf of section.leaves) {
      paths.add(leaf.path);
    }
    for (const list of section.lists) {
      paths.add(list.path);
      for (const column of list.columns) {
        if (column.kind === 'field') {
          paths.add(`${list.path}.${column.name}`);
        } else {
          for (const option of column.options) {
            paths.add(`${list.path}.${option.name}`);
          }
        }
      }
    }
  }
  for (const action of template.actions ?? []) {
    paths.add(action.id);
  }
  return { paths, rows, sections };
}

function columnPaths(piece: Extract<PresetPiece, { kind: 'recordTable' }>): readonly string[] {
  return (piece.columns ?? []).map((column) => `${piece.list}.${column}`);
}

function piecePaths(piece: PresetPiece): readonly string[] {
  switch (piece.kind) {
    case 'statPills':
      return piece.pills.flatMap((pill) =>
        pill.row === undefined ? [pill.path] : [pill.path, pill.row],
      );
    case 'pips':
      return typeof piece.max === 'string' ? [piece.count, piece.max] : [piece.count];
    case 'spellSlots':
      return [
        ...piece.levels.flatMap((level) => [level.count, level.max]),
        ...(piece.pact === undefined ? [] : [piece.pact.count, piece.pact.max]),
      ];
    case 'abilityGrid':
      return piece.abilities.flatMap((ability) => [
        ability.score,
        ability.modifier,
        ...(ability.row === undefined ? [] : [ability.row]),
      ]);
    case 'saveList':
      return piece.saves.flatMap((save) => [
        save.proficiency,
        save.total,
        ...(save.row === undefined ? [] : [save.row]),
      ]);
    case 'skillList':
      return [
        ...piece.skills.flatMap((skill) => [
          skill.rank,
          skill.total,
          ...(skill.row === undefined ? [] : [skill.row]),
        ]),
        ...(typeof piece.max === 'string' ? [piece.max] : []),
      ];
    case 'hitPoints':
      return [
        piece.current,
        ...(typeof piece.max === 'string' ? [piece.max] : []),
        ...(piece.temp === undefined ? [] : [piece.temp]),
      ];
    case 'conditions':
      return [
        ...piece.conditions.map((flag) => flag.path),
        ...(piece.exhaustion === undefined
          ? []
          : [
              piece.exhaustion.level,
              ...(typeof piece.exhaustion.max === 'string' ? [piece.exhaustion.max] : []),
              ...(piece.exhaustion.penalty === undefined ? [] : [piece.exhaustion.penalty]),
            ]),
      ];
    case 'recordTable':
      return [piece.list, ...columnPaths(piece)];
    case 'resetBar':
      return piece.actions;
    case 'rollMode':
      return [];
    case 'textFields':
      return piece.fields.map((field) => field.path);
  }
}

function pieceSections(piece: PresetPiece): readonly string[] {
  switch (piece.kind) {
    case 'abilityGrid':
    case 'saveList':
    case 'skillList':
      return [piece.section];
    default:
      return [];
  }
}

function pieceRows(piece: PresetPiece): readonly string[] {
  switch (piece.kind) {
    case 'statPills':
      return piece.pills.flatMap((pill) => (pill.row === undefined ? [] : [pill.row]));
    case 'abilityGrid':
      return piece.abilities.flatMap((ability) => (ability.row === undefined ? [] : [ability.row]));
    case 'saveList':
      return piece.saves.flatMap((save) => (save.row === undefined ? [] : [save.row]));
    case 'skillList':
      return piece.skills.flatMap((skill) => (skill.row === undefined ? [] : [skill.row]));
    default:
      return [];
  }
}

function layoutKeys(config: PresetConfig): readonly string[] {
  const keys: string[] = [];
  for (const blocks of [config.layout ?? [], config.narrowLayout ?? []]) {
    for (const block of blocks) {
      if ('columns' in block) {
        for (const column of block.columns) {
          keys.push(...column);
        }
      } else {
        keys.push(...block.sections);
      }
    }
  }
  return keys;
}

function boundPaths(config: PresetConfig): readonly string[] {
  return [
    config.header.name,
    ...(config.header.subtitle === undefined ? [] : [config.header.subtitle]),
    ...config.pieces.flatMap(piecePaths),
  ];
}

export function configPathErrors(
  config: PresetConfig,
  template: TemplateDetail,
): readonly string[] {
  const known = knownTemplate(template);
  const errors: string[] = [];
  const report = (value: string) => {
    if (!errors.includes(value)) {
      errors.push(value);
    }
  };
  for (const path of boundPaths(config)) {
    if (!known.paths.has(path)) {
      report(path);
    }
  }
  for (const piece of config.pieces) {
    for (const section of pieceSections(piece)) {
      if (!known.sections.has(section)) {
        report(section);
      }
    }
    for (const row of pieceRows(piece)) {
      if (!known.rows.has(row)) {
        report(row);
      }
    }
  }
  const pieceKeys = new Set(config.pieces.map((piece) => piece.key));
  for (const key of layoutKeys(config)) {
    if (!pieceKeys.has(key)) {
      report(key);
    }
  }
  return errors;
}
