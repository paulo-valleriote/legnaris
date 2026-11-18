import { useState } from 'react';
import type { FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy } from '../api/error_mapper';
import type { Campaign } from '../api/models/campaign';
import { useApiClient } from '../app/providers';
import { message } from '../l10n/strings';
import { Button } from '../ui/button';
import { Field } from '../ui/field';
import { ErrorState } from '../ui/state';
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
          <Field
            label={message('invite.passwordLabel')}
            id="campaign-password"
            type="password"
            autoComplete="new-password"
            value={password}
            disabled={saving}
            hint={message('invite.passwordHint')}
            error={passwordError}
            onChange={setPassword}
          />
          <div className="lg-campaign__actions">
            <Button
              type="submit"
              label={message('invite.passwordSave')}
              loading={saving}
              disabled={password === ''}
            />
            {campaign.passwordSet === true ? (
              <Button
                variant="outline"
                label={message('invite.passwordClear')}
                disabled={saving}
                onClick={() => clearPassword.mutate()}
              />
            ) : null}
          </div>
        </form>
      </div>

      <div className="lg-campaign__settings-group">
        <h3 className="lg-campaign__subheading">{message('campaign.dangerHeading')}</h3>
        <p className="lg-campaign__meta">{message('campaign.deleteNote')}</p>
        <div className="lg-campaign__actions">
          <Button
            variant="danger"
            label={message('campaign.delete')}
            disabled={busy || deleting}
            onClick={onDelete}
          />
        </div>
      </div>

      {error ? <ErrorState message={error} /> : null}
    </section>
  );
}
