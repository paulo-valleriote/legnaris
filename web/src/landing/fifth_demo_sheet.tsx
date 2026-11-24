import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import { PresetPlayView } from '../sheet/preset/play_view';
import { DemoProvider } from './demo_provider';
import { NO_DERIVATION } from './demo_store';
import { FIFTH_DEMO_NOTE, FIFTH_DEMO_SHEET, FIFTH_DEMO_TEMPLATE } from './fifth_demo_character';
import './demo_page.css';

const fifthCharacterPreset = presetFromConfig(fifthCharacterConfig);

export default function FifthDemoSheet() {
  return (
    <DemoProvider template={FIFTH_DEMO_TEMPLATE} sheet={FIFTH_DEMO_SHEET} derive={NO_DERIVATION}>
      <div className="lg-demo-sheet" data-testid="landing-fifth-demo-sheet">
        <fifthCharacterPreset.Header level={2} />
        <PresetPlayView preset={fifthCharacterPreset} />
        <p className="lg-demo__lead" data-testid="landing-fifth-demo-note">
          {FIFTH_DEMO_NOTE}
        </p>
      </div>
    </DemoProvider>
  );
}
