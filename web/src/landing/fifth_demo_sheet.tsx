import type { ReactNode } from 'react';
import { message } from '../l10n/strings';
import { PendingCardOverrideProvider } from '../sheet/preset/pending_card';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import { PresetPlayView } from '../sheet/preset/play_view';
import { DemoProvider } from './demo_provider';
import { FIFTH_DEMO_SHEET, FIFTH_DEMO_TEMPLATE } from './fifth_demo_character';
import { templateDerivation } from './template_derivation';
import './demo_page.css';

const fifthCharacterPreset = presetFromConfig(fifthCharacterConfig);

const fifthCharacterDerivation = templateDerivation(FIFTH_DEMO_TEMPLATE);

interface FifthDemoSheetProps {
  readonly onInteract?: () => void;
  readonly pendingAction?: ReactNode;
}

export default function FifthDemoSheet({ onInteract, pendingAction }: FifthDemoSheetProps) {
  const sheet = (
    <DemoProvider
      template={FIFTH_DEMO_TEMPLATE}
      sheet={FIFTH_DEMO_SHEET}
      derive={fifthCharacterDerivation}
      onInteract={onInteract}
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
  if (pendingAction === undefined) {
    return sheet;
  }
  return (
    <PendingCardOverrideProvider
      value={{ note: message('demo.pendingNote'), action: pendingAction }}
    >
      {sheet}
    </PendingCardOverrideProvider>
  );
}
