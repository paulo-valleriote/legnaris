import type { ComponentType } from 'react';
import type { PresetHeaderProps, SheetPreset } from '../types';
import { IdentityHeader } from './identity_header';
import { Pips } from './pips';
import { RecordTable } from './record_table';
import { ResetBar } from './reset_bar';
import { RollModeBar } from './roll_mode_bar';
import { StatPills } from './stat_pills';
import type {
  PresetConfig,
  PresetPiece,
  PresetPipsPiece,
  PresetRecordTablePiece,
  PresetResetBarPiece,
  PresetRollModePiece,
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

function recordTableComponent(piece: PresetRecordTablePiece): ComponentType {
  return function RecordTableSection() {
    return <RecordTable piece={piece} />;
  };
}

function resetBarComponent(piece: PresetResetBarPiece): ComponentType {
  return function ResetBarSection() {
    return <ResetBar piece={piece} />;
  };
}

function rollModeComponent(piece: PresetRollModePiece): ComponentType {
  return function RollModeSection() {
    return <RollModeBar piece={piece} />;
  };
}

function pieceComponent(piece: PresetPiece): ComponentType {
  switch (piece.kind) {
    case 'statPills':
      return statPillsComponent(piece);
    case 'pips':
      return pipsComponent(piece);
    case 'recordTable':
      return recordTableComponent(piece);
    case 'resetBar':
      return resetBarComponent(piece);
    case 'rollMode':
      return rollModeComponent(piece);
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
