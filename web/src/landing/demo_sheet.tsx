import { fantasyPreset } from '../sheet/preset/fantasy';
import { PresetPlayView } from '../sheet/preset/play_view';
import { DemoProvider } from './demo_provider';

export default function DemoSheet() {
  return (
    <DemoProvider>
      <div className="lg-demo-sheet" data-testid="landing-demo-sheet">
        <fantasyPreset.Header />
        <PresetPlayView preset={fantasyPreset} />
      </div>
    </DemoProvider>
  );
}
