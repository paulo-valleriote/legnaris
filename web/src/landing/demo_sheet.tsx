import { fantasyPreset } from '../sheet/preset/fantasy';
import { PresetPlayView } from '../sheet/preset/play_view';
import { DemoProvider } from './demo_provider';

export interface DemoSheetProps {
  readonly onInteract?: () => void;
}

export default function DemoSheet({ onInteract }: DemoSheetProps) {
  return (
    <DemoProvider onInteract={onInteract}>
      <div className="lg-demo-sheet" data-testid="landing-demo-sheet">
        <fantasyPreset.Header level={2} />
        <PresetPlayView preset={fantasyPreset} />
      </div>
    </DemoProvider>
  );
}
