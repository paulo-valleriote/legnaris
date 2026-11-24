import { message } from '../l10n/strings';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import { PresetPlayView } from '../sheet/preset/play_view';
import { DemoProvider } from './demo_provider';
import { FIFTH_DEMO_SHEET, FIFTH_DEMO_TEMPLATE } from './fifth_demo_character';
import { templateDerivation } from './template_derivation';
import './demo_page.css';

const fifthCharacterPreset = presetFromConfig(fifthCharacterConfig);

const fifthCharacterDerivation = templateDerivation(FIFTH_DEMO_TEMPLATE);

export default function FifthDemoSheet() {
  return (
    <DemoProvider
      template={FIFTH_DEMO_TEMPLATE}
      sheet={FIFTH_DEMO_SHEET}
      derive={fifthCharacterDerivation}
    >
      <div className="lg-demo-sheet" data-testid="landing-fifth-demo-sheet">
        <fifthCharacterPreset.Header level={2} />
        <PresetPlayView preset={fifthCharacterPreset} />
        <p className="lg-demo__lead" data-testid="landing-fifth-demo-note">
          {message('landing.fifthDemoNote')}
        </p>
      </div>
    </DemoProvider>
  );
}
