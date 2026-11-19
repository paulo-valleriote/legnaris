import { HitPointsCard } from './hp_card';

export function CombatStrip() {
  return (
    <div className="lg-combat-strip" data-testid="sheet-combat-strip">
      <HitPointsCard />
    </div>
  );
}
