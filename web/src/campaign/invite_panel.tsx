import { useState } from 'react';
import type { FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy } from '../api/error_mapper';
import type { Campaign } from '../api/models/campaign';
import { useApiClient } from '../app/providers';
import { message } from '../l10n/strings';
import { Dialog } from '../ui/dialog';
import { CAMPAIGNS_QUERY_KEY, seedCampaign } from './campaign_queries';

const PASSWORD_MIN_RUNES = 8;

export function inviteLink(origin: string, code: string): string {
  return `${origin}/join/${encodeURIComponent(code)}`;
}

export function InvitePanel({ campaign }: { campaign: Campaign }) {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const [rotateOpen, setRotateOpen] = useState(false);
  const [password, setPassword] = useState('');
  const [passwordError, setPasswordError] = useState<string | undefined>(undefined);
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
      onDone(next);
    },
    onError: (failure) => {
      setRotateOpen(false);
      setError(failureCopy(failure));
    },
  });

  const savePassword = useMutation({
    mutationFn: (value: string) => client.setCampaignPassword(campaign.id, value),
    onSuccess: (next) => {
      setPassword('');
      onDone(next);
    },
    onError: (failure) => setError(failureCopy(failure)),
  });

  const clearPassword = useMutation({
    mutationFn: () => client.clearCampaignPassword(campaign.id),
    onSuccess: onDone,
    onError: (failure) => setError(failureCopy(failure)),
  });

  const busy = rotate.isPending || savePassword.isPending || clearPassword.isPending;
  const link =
    campaign.inviteCode === undefined
      ? undefined
      : inviteLink(window.location.origin, campaign.inviteCode);

  const submitPassword = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if ([...password].length < PASSWORD_MIN_RUNES) {
      setPasswordError(message('invite.passwordTooShort'));
      return;
    }
    setPasswordError(undefined);
    savePassword.mutate(password);
  };

  return (
    <section className="lg-campaign__section" aria-labelledby="campaign-invite-heading">
      <h2 className="lg-campaign__heading" id="campaign-invite-heading">
        {message('invite.heading')}
      </h2>
      {campaign.inviteCode === undefined || link === undefined ? (
        <p className="lg-campaign__message">{message('invite.none')}</p>
      ) : (
        <dl className="lg-campaign__fields">
          <div className="lg-campaign__field">
            <dt className="lg-campaign__name">{message('invite.codeLabel')}</dt>
            <dd className="lg-campaign__value">{campaign.inviteCode}</dd>
          </div>
          <div className="lg-campaign__field">
            <dt className="lg-campaign__name">{message('invite.linkLabel')}</dt>
            <dd className="lg-campaign__value">
              <a href={link}>{link}</a>
            </dd>
          </div>
        </dl>
      )}
      <div className="lg-campaign__actions">
        <button
          type="button"
          className="lg-button lg-button--outline lg-button--md"
          disabled={busy}
          onClick={() => setRotateOpen(true)}
        >
          {message('invite.rotate')}
        </button>
      </div>

      <h3 className="lg-campaign__subheading">{message('invite.passwordHeading')}</h3>
      <p className="lg-campaign__meta">
        {campaign.passwordSet === true
          ? message('invite.passwordSet')
          : message('invite.passwordNone')}
      </p>
      <form className="lg-campaign__form" onSubmit={submitPassword} noValidate>
        <label className="lg-field" htmlFor="campaign-password">
          <span className="lg-field__label">{message('invite.passwordLabel')}</span>
          <input
            id="campaign-password"
            className="lg-field__input"
            type="password"
            autoComplete="new-password"
            value={password}
            disabled={busy}
            aria-describedby="campaign-password-hint"
            aria-invalid={passwordError === undefined ? undefined : true}
            onChange={(event) => setPassword(event.target.value)}
          />
          <span id="campaign-password-hint" className="lg-campaign__meta">
            {message('invite.passwordHint')}
          </span>
          {passwordError === undefined ? null : (
            <span className="lg-campaign__error">{passwordError}</span>
          )}
        </label>
        <div className="lg-campaign__actions">
          <button
            type="submit"
            className="lg-button lg-button--primary lg-button--md"
            disabled={busy || password === ''}
          >
            {message('invite.passwordSave')}
          </button>
          {campaign.passwordSet === true ? (
            <button
              type="button"
              className="lg-button lg-button--outline lg-button--md"
              disabled={busy}
              onClick={() => clearPassword.mutate()}
            >
              {message('invite.passwordClear')}
            </button>
          ) : null}
        </div>
      </form>

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
