import { PendingChangesCard } from './pending_card';
import { usePresetContext } from './preset_context';
import type { SheetPreset } from './types';
import './preset.css';

export function PresetPlayView({ preset }: { readonly preset: SheetPreset }) {
  const { sheet } = usePresetContext();
  const statBlock = preset.statBlock?.when(sheet.kind) ? preset.statBlock : undefined;

  return (
    <div className="lg-preset" data-testid="sheet-play">
      {statBlock === undefined ? (
        preset.sections.map(({ key, Component }) => <Component key={key} />)
      ) : (
        <statBlock.Body />
      )}
      <PendingChangesCard />
    </div>
  );
}
