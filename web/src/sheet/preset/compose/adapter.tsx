import type { ComponentType } from 'react';
import type { PresetHeaderProps, SheetPreset } from '../types';
import { AbilityGrid } from './ability_grid';
import { Conditions } from './conditions';
import { HitPointsTrack } from './hit_points_track';
import { IdentityHeader } from './identity_header';
import { Pips } from './pips';
import { SaveList } from './save_list';
import { SkillList } from './skill_list';
import { StatPills } from './stat_pills';
import type {
  PresetAbilityGridPiece,
  PresetConfig,
  PresetPiece,
  PresetPipsPiece,
  PresetSaveListPiece,
  PresetSkillListPiece,
  PresetConditionsPiece,
  PresetHitPointsPiece,
  PresetStatPillsPiece,
} from './types';

function headerComponent(config: PresetConfig): ComponentType<PresetHeaderProps> {
  return function ConfigHeader({ level }: PresetHeaderProps) {
    return <IdentityHeader header={config.header} level={level} />;
  };
}

function statPillsComponent(piece: PresetStatPillsPiece): ComponentType {
  return function StatPillsSection() {
    return <StatPills piece={piece} />;
  };
}

function pipsComponent(piece: PresetPipsPiece): ComponentType {
  return function PipsSection() {
    return <Pips piece={piece} />;
  };
}

function abilityGridComponent(piece: PresetAbilityGridPiece): ComponentType {
  return function AbilityGridSection() {
    return <AbilityGrid piece={piece} />;
  };
}

function saveListComponent(piece: PresetSaveListPiece): ComponentType {
  return function SaveListSection() {
    return <SaveList piece={piece} />;
  };
}

function skillListComponent(piece: PresetSkillListPiece): ComponentType {
  return function SkillListSection() {
    return <SkillList piece={piece} />;
  };
}

function hitPointsComponent(piece: PresetHitPointsPiece): ComponentType {
  return function HitPointsSection() {
    return <HitPointsTrack piece={piece} />;
  };
}

function conditionsComponent(piece: PresetConditionsPiece): ComponentType {
  return function ConditionsSection() {
    return <Conditions piece={piece} />;
  };
}

function pieceComponent(piece: PresetPiece): ComponentType {
  switch (piece.kind) {
    case 'statPills':
      return statPillsComponent(piece);
    case 'pips':
      return pipsComponent(piece);
    case 'abilityGrid':
      return abilityGridComponent(piece);
    case 'saveList':
      return saveListComponent(piece);
    case 'skillList':
      return skillListComponent(piece);
    case 'hitPoints':
      return hitPointsComponent(piece);
    case 'conditions':
      return conditionsComponent(piece);
  }
}

export function presetFromConfig(config: PresetConfig): SheetPreset {
  return {
    templateId: config.templateId,
    Header: headerComponent(config),
    sections: config.pieces.map((piece) => ({ key: piece.key, Component: pieceComponent(piece) })),
    ...(config.layout === undefined ? {} : { layout: config.layout }),
    ...(config.narrowLayout === undefined ? {} : { narrowLayout: config.narrowLayout }),
    ...(config.sectionNav === undefined ? {} : { sectionNav: config.sectionNav }),
  };
}
