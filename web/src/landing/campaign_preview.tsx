import { useId, useState } from 'react';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';

interface PreviewSheet {
  readonly name: string;
  readonly player: string;
}

const PREVIEW_SHEETS: readonly PreviewSheet[] = [
  { name: 'Mira Thornfield', player: 'sam' },
  { name: 'Odo Brightwater', player: 'rin' },
];

const PREVIEW_NPC = 'Goblin Boss';

export function CampaignPreview() {
  const [revealed, setRevealed] = useState(false);
  const [notice, setNotice] = useState('');
  const sheetsId = useId();
  const npcsId = useId();

  const toggle = () => {
    const next = !revealed;
    setRevealed(next);
    setNotice(message(next ? 'campaign.npcRevealedNotice' : 'campaign.npcHiddenNotice'));
  };

  return (
    <div
      className="lg-gm-preview"
      role="group"
      aria-label={message('landing.gmPreviewLabel')}
      data-testid="landing-gm-preview"
    >
      <p className="lg-gm-preview__name">{message('landing.gmPreviewCampaign')}</p>
      <p className="lg-gm-preview__hint">{message('landing.gmPreviewHint')}</p>

      <h3 className="lg-gm-preview__heading" id={sheetsId}>
        {message('campaign.sheetsHeading')}
      </h3>
      <ul className="lg-gm-preview__list" aria-labelledby={sheetsId}>
        {PREVIEW_SHEETS.map((sheet) => (
          <li className="lg-gm-preview__row" key={sheet.name}>
            <span className="lg-gm-preview__sheet">{sheet.name}</span>
            <span className="lg-gm-preview__meta">
              {messageWith('campaign.playedBy', { name: sheet.player })}
            </span>
          </li>
        ))}
      </ul>

      <h3 className="lg-gm-preview__heading" id={npcsId}>
        {message('campaign.npcsHeading')}
      </h3>
      <ul className="lg-gm-preview__list" aria-labelledby={npcsId}>
        <li className="lg-gm-preview__row lg-gm-preview__row--npc">
          <span className="lg-gm-preview__sheet">{PREVIEW_NPC}</span>
          <span
            className={`lg-gm-preview__visibility${revealed ? ' lg-gm-preview__visibility--on' : ''}`}
            data-testid="landing-gm-preview-visibility"
          >
            {message(revealed ? 'campaign.npcVisible' : 'campaign.npcHidden')}
          </span>
          <Button
            className="lg-gm-preview__toggle"
            variant="outline"
            size="sm"
            label={message(revealed ? 'campaign.hideNpcAction' : 'campaign.revealNpcAction')}
            name={messageWith('campaign.revealNpc', { name: PREVIEW_NPC })}
            ariaPressed={revealed}
            onClick={toggle}
          />
        </li>
      </ul>

      <p className="lg-gm-preview__notice" role="status" aria-live="polite" aria-atomic="true">
        {notice}
      </p>
    </div>
  );
}
