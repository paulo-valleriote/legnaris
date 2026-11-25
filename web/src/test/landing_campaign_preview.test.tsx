import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message, messageWith } from '../l10n/strings';
import { CampaignPreview } from '../landing/campaign_preview';

function preview() {
  return screen.getByTestId('landing-gm-preview');
}

function revealButton() {
  return within(preview()).getByRole('button', {
    name: messageWith('campaign.revealNpc', { name: 'Goblin Boss' }),
  });
}

function notice() {
  return within(preview()).getByRole('status');
}

describe('the landing Campaign preview', () => {
  it('Happy path: it shows the linked player sheets and the NPC list as the game master sees them', () => {
    render(<CampaignPreview />);

    expect(preview()).toHaveAccessibleName(message('landing.gmPreviewLabel'));
    const sheets = within(preview()).getByRole('list', {
      name: message('campaign.sheetsHeading'),
    });
    expect(within(sheets).getAllByRole('listitem')).toHaveLength(2);
    expect(
      within(sheets).getByText(messageWith('campaign.playedBy', { name: 'sam' })),
    ).toBeInTheDocument();
    expect(
      within(preview()).getByRole('list', { name: message('campaign.npcsHeading') }),
    ).toBeInTheDocument();
    expect(within(preview()).getByText(message('landing.gmPreviewHint'))).toBeInTheDocument();
  });

  it('Edge: the NPC starts hidden from players with an empty notice', () => {
    render(<CampaignPreview />);

    expect(screen.getByTestId('landing-gm-preview-visibility')).toHaveTextContent(
      message('campaign.npcHidden'),
    );
    expect(revealButton()).toHaveAttribute('aria-pressed', 'false');
    expect(revealButton()).toHaveTextContent(message('campaign.revealNpcAction'));
    expect(notice()).toBeEmptyDOMElement();
  });

  it('Happy path: revealing the NPC shows it to players and announces it', async () => {
    const user = userEvent.setup();
    render(<CampaignPreview />);

    await user.click(revealButton());

    expect(screen.getByTestId('landing-gm-preview-visibility')).toHaveTextContent(
      message('campaign.npcVisible'),
    );
    expect(revealButton()).toHaveAttribute('aria-pressed', 'true');
    expect(revealButton()).toHaveTextContent(message('campaign.hideNpcAction'));
    expect(notice()).toHaveTextContent(message('campaign.npcRevealedNotice'));
    expect(notice()).toHaveAttribute('aria-live', 'polite');
  });

  it('Edge: hiding the NPC again puts it back and announces the change', async () => {
    const user = userEvent.setup();
    render(<CampaignPreview />);

    await user.click(revealButton());
    await user.click(revealButton());

    expect(screen.getByTestId('landing-gm-preview-visibility')).toHaveTextContent(
      message('campaign.npcHidden'),
    );
    expect(revealButton()).toHaveAttribute('aria-pressed', 'false');
    expect(notice()).toHaveTextContent(message('campaign.npcHiddenNotice'));
  });

  it('Edge: the toggle works from the keyboard', async () => {
    const user = userEvent.setup();
    render(<CampaignPreview />);

    revealButton().focus();
    await user.keyboard('{Enter}');

    expect(revealButton()).toHaveAttribute('aria-pressed', 'true');
  });
});
