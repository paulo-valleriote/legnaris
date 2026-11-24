import { buildSections } from '../../../engine/field_grouping';
import type { TemplateDetail } from '../../../api/models/template';
import type { PresetConfig, PresetPiece } from './types';

function knownPaths(template: TemplateDetail): ReadonlySet<string> {
  const paths = new Set<string>();
  for (const section of buildSections(template.fields)) {
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
  return paths;
}

function columnPaths(piece: Extract<PresetPiece, { kind: 'recordTable' }>): readonly string[] {
  return (piece.columns ?? []).map((column) => `${piece.list}.${column}`);
}

function piecePaths(piece: PresetPiece): readonly string[] {
  switch (piece.kind) {
    case 'statPills':
      return piece.pills.map((pill) => pill.path);
    case 'pips':
      return typeof piece.max === 'string' ? [piece.count, piece.max] : [piece.count];
    case 'recordTable':
      return [piece.list, ...columnPaths(piece)];
    case 'resetBar':
      return piece.actions;
    case 'rollMode':
      return [];
  }
}

function boundPaths(config: PresetConfig): readonly string[] {
  return [
    config.header.name,
    ...(config.header.subtitle === undefined ? [] : [config.header.subtitle]),
    ...config.pieces.flatMap(piecePaths),
    ...(config.resetActions ?? []),
  ];
}

export function configPathErrors(
  config: PresetConfig,
  template: TemplateDetail,
): readonly string[] {
  const known = knownPaths(template);
  const errors: string[] = [];
  for (const path of boundPaths(config)) {
    if (!known.has(path) && !errors.includes(path)) {
      errors.push(path);
    }
  }
  return errors;
}
