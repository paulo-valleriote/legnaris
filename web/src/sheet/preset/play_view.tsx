import { PendingChangesCard } from './pending_card';
import type { SheetPreset } from './types';
import './preset.css';

export function PresetPlayView({ preset }: { readonly preset: SheetPreset }) {
  return (
    <div className="lg-preset" data-testid="sheet-play">
      {preset.sections.map(({ key, Component }) => (
        <Component key={key} />
      ))}
      <PendingChangesCard />
    </div>
  );
}
