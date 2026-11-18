import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy } from '../api/error_mapper';
import type { Campaign } from '../api/models/campaign';
import { useApiClient } from '../app/providers';
import { message } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import { Field } from '../ui/field';
import { Notice } from '../ui/notice';
import { EmptyState, ErrorState } from '../ui/state';
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
        <EmptyState message={message('invite.none')} />
      ) : (
        <>
          <div className="lg-campaign__invite">
            <div className="lg-campaign__invite-row">
              <div className="lg-campaign__invite-field">
                <Field
                  label={message('invite.linkLabel')}
                  id="campaign-invite-link"
                  value={link}
                  readOnly
                />
              </div>
              <Button
                variant="outline"
                label={message('invite.copyLink')}
                onClick={() => void copy()}
              />
            </div>
            {copyResult === 'copied' ? (
              <Notice tone="success" live="polite">
                {message('invite.copied')}
              </Notice>
            ) : null}
            {copyResult === 'failed' ? <ErrorState message={message('invite.copyFailed')} /> : null}
          </div>
          <p className="lg-campaign__code">
            <span className="lg-campaign__code-label">{message('invite.codeLabel')}</span>
            <span className="lg-campaign__code-value">{campaign.inviteCode}</span>
          </p>
        </>
      )}
      <div className="lg-campaign__actions">
        <Button
          variant="outline"
          label={message('invite.rotate')}
          disabled={rotate.isPending}
          onClick={() => setRotateOpen(true)}
        />
      </div>

      {error ? <ErrorState message={error} /> : null}

      <Dialog
        open={rotateOpen}
        title={message('invite.rotateTitle')}
        description={message('invite.rotateMessage')}
        closeLabel={message('common.close')}
        onClose={() => {
          if (!rotate.isPending) setRotateOpen(false);
        }}
        actions={
          <>
            <Button
              variant="outline"
              label={message('invite.keepCode')}
              disabled={rotate.isPending}
              onClick={() => setRotateOpen(false)}
            />
            <Button
              variant="danger"
              label={message('invite.rotateConfirm')}
              loading={rotate.isPending}
              onClick={() => rotate.mutate()}
            />
          </>
        }
      />
    </section>
  );
}
