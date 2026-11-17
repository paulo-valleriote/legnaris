import { useState } from 'react';
import type { FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy } from '../api/error_mapper';
import type { Campaign } from '../api/models/campaign';
import { useApiClient } from '../app/providers';
import { message } from '../l10n/strings';
import { CAMPAIGNS_QUERY_KEY, seedCampaign } from './campaign_queries';

export const SETTINGS_PANEL_ID = 'campaign-settings-panel';

const PASSWORD_MIN_RUNES = 8;

export function CampaignSettings({
  campaign,
  busy,
  deleting,
  onDelete,
}: {
  campaign: Campaign;
  busy: boolean;
  deleting: boolean;
  onDelete: () => void;
}) {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const [password, setPassword] = useState('');
  const [passwordError, setPasswordError] = useState<string | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);

  const onDone = (next: Campaign) => {
    setError(undefined);
    seedCampaign(queryClient, next);
    void queryClient.invalidateQueries({ queryKey: CAMPAIGNS_QUERY_KEY });
  };

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

  const saving = savePassword.isPending || clearPassword.isPending;

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
    <section
      className="lg-campaign__panel"
      id={SETTINGS_PANEL_ID}
      aria-labelledby="campaign-settings-heading"
    >
      <h2 className="lg-campaign__heading" id="campaign-settings-heading">
        {message('campaign.settingsHeading')}
      </h2>

      <div className="lg-campaign__settings-group">
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
              disabled={saving}
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
              disabled={saving || password === ''}
            >
              {message('invite.passwordSave')}
            </button>
            {campaign.passwordSet === true ? (
              <button
                type="button"
                className="lg-button lg-button--outline lg-button--md"
                disabled={saving}
                onClick={() => clearPassword.mutate()}
              >
                {message('invite.passwordClear')}
              </button>
            ) : null}
          </div>
        </form>
      </div>

      <div className="lg-campaign__settings-group">
        <h3 className="lg-campaign__subheading">{message('campaign.dangerHeading')}</h3>
        <p className="lg-campaign__meta">{message('campaign.deleteNote')}</p>
        <div className="lg-campaign__actions">
          <button
            type="button"
            className="lg-button lg-button--danger lg-button--md"
            disabled={busy || deleting}
            onClick={onDelete}
          >
            {message('campaign.delete')}
          </button>
        </div>
      </div>

      {error ? (
        <p className="lg-campaign__error" role="alert">
          {error}
        </p>
      ) : null}
    </section>
  );
}
