import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy } from '../api/error_mapper';
import type { Campaign } from '../api/models/campaign';
import { useApiClient } from '../app/providers';
import { message } from '../l10n/strings';
import { Dialog } from '../ui/dialog';
import { CAMPAIGNS_QUERY_KEY, seedCampaign } from './campaign_queries';

export const INVITE_PANEL_ID = 'campaign-invite-panel';

export function inviteLink(origin: string, code: string): string {
  return `${origin}/join/${encodeURIComponent(code)}`;
}

type CopyResult = 'copied' | 'failed';

export function InvitePanel({ campaign }: { campaign: Campaign }) {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const [rotateOpen, setRotateOpen] = useState(false);
  const [copyResult, setCopyResult] = useState<CopyResult | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);

  const onDone = (next: Campaign) => {
    setError(undefined);
    seedCampaign(queryClient, next);
    void queryClient.invalidateQueries({ queryKey: CAMPAIGNS_QUERY_KEY });
  };

  const rotate = useMutation({
    mutationFn: () => client.rotateInvite(campaign.id),
    onSuccess: (next) => {
      setRotateOpen(false);
      setCopyResult(undefined);
      onDone(next);
    },
    onError: (failure) => {
      setRotateOpen(false);
      setError(failureCopy(failure));
    },
  });

  const link =
    campaign.inviteCode === undefined
      ? undefined
      : inviteLink(window.location.origin, campaign.inviteCode);

  const copy = async () => {
    if (link === undefined) {
      return;
    }
    try {
      await navigator.clipboard.writeText(link);
      setCopyResult('copied');
    } catch {
      setCopyResult('failed');
    }
  };

  return (
    <section
      className="lg-campaign__panel"
      id={INVITE_PANEL_ID}
      aria-labelledby="campaign-invite-heading"
    >
      <h2 className="lg-campaign__heading" id="campaign-invite-heading">
        {message('invite.heading')}
      </h2>
      {campaign.inviteCode === undefined || link === undefined ? (
        <p className="lg-campaign__empty">{message('invite.none')}</p>
      ) : (
        <>
          <div className="lg-campaign__invite">
            <p className="lg-campaign__label">{message('invite.linkLabel')}</p>
            <div className="lg-campaign__invite-row">
              <input
                className="lg-campaign__invite-link"
                type="text"
                readOnly
                value={link}
                aria-label={message('invite.linkLabel')}
              />
              <button
                type="button"
                className="lg-button lg-button--outline lg-button--md"
                onClick={() => void copy()}
              >
                {message('invite.copyLink')}
              </button>
            </div>
            {copyResult === 'copied' ? (
              <p className="lg-campaign__message" role="status">
                {message('invite.copied')}
              </p>
            ) : null}
            {copyResult === 'failed' ? (
              <p className="lg-campaign__error" role="alert">
                {message('invite.copyFailed')}
              </p>
            ) : null}
          </div>
          <p className="lg-campaign__code">
            <span className="lg-campaign__code-label">{message('invite.codeLabel')}</span>
            <span className="lg-campaign__code-value">{campaign.inviteCode}</span>
          </p>
        </>
      )}
      <div className="lg-campaign__actions">
        <button
          type="button"
          className="lg-button lg-button--outline lg-button--md"
          disabled={rotate.isPending}
          onClick={() => setRotateOpen(true)}
        >
          {message('invite.rotate')}
        </button>
      </div>

      {error ? (
        <p className="lg-campaign__error" role="alert">
          {error}
        </p>
      ) : null}

      <Dialog
        open={rotateOpen}
        title={message('invite.rotateTitle')}
        description={message('invite.rotateMessage')}
        onClose={() => {
          if (!rotate.isPending) setRotateOpen(false);
        }}
      >
        <div className="lg-campaign__actions">
          <button
            type="button"
            className="lg-button lg-button--outline lg-button--md"
            disabled={rotate.isPending}
            onClick={() => setRotateOpen(false)}
          >
            {message('invite.keepCode')}
          </button>
          <button
            type="button"
            className="lg-button lg-button--danger lg-button--md"
            disabled={rotate.isPending}
            onClick={() => rotate.mutate()}
          >
            {message('invite.rotateConfirm')}
          </button>
        </div>
      </Dialog>
    </section>
  );
}
