import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from 'react-router-dom';
import type { Campaign, CampaignInvite } from '../api/models/campaign';
import { isSheetLimitReached } from '../api/models/plan';
import { failureCopy, failureKind } from '../api/error_mapper';
import { useApiClient } from '../app/providers';
import { CreateSheetDialog } from '../catalog/create_sheet_dialog';
import {
  CAMPAIGNS_QUERY_KEY,
  seedCampaign,
  useOwnedCharacterSheets,
} from '../campaign/campaign_queries';
import { usePlanUsage } from '../catalog/queries';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { ErrorState, LoadingState } from '../ui/state';
import { Field } from '../ui/field';
import { Notice } from '../ui/notice';
import { Select } from '../ui/select';
import './join.css';

interface JoinCardProps {
  readonly children: ReactNode;
  readonly title?: string;
  readonly lead?: string;
}

function JoinCard({ children, title, lead }: JoinCardProps) {
  return (
    <main className="lg-join">
      <section className="lg-join__card" aria-label={message('join.title')}>
        {title === undefined ? null : <h1 className="lg-join__title">{title}</h1>}
        {lead === undefined ? null : <p className="lg-join__lead">{lead}</p>}
        {children}
      </section>
    </main>
  );
}

function playerCount(count: number): string {
  return count === 1 ? message('join.playersOne') : messageWith('join.players', { count });
}

export function JoinScreen() {
  const { code = '' } = useParams();
  const client = useApiClient();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [password, setPassword] = useState('');
  const [passwordError, setPasswordError] = useState<string | undefined>(undefined);
  const [previewInvite, setPreviewInvite] = useState<CampaignInvite | undefined>(undefined);
  const [joinError, setJoinError] = useState<string | undefined>(undefined);
  const [sheetId, setSheetId] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const autoJoined = useRef(false);

  const inviteQuery = useQuery({
    queryKey: ['campaign-invite', code],
    queryFn: () => client.getCampaignInvite(code),
    retry: false,
  });
  const invite = previewInvite ?? inviteQuery.data;
  const passwordRequired =
    inviteQuery.isError && failureKind(inviteQuery.error) === 'passwordRequired';
  const invalidInvite =
    inviteQuery.isError &&
    (failureKind(inviteQuery.error) === 'invalidInvite' ||
      failureKind(inviteQuery.error) === 'notFound' ||
      failureKind(inviteQuery.error) === 'campaignNotFound');
  const plan = usePlanUsage();
  const owned = useOwnedCharacterSheets(invite !== undefined);
  const eligibleSheets = (owned.data ?? []).filter(
    (sheet) =>
      invite !== undefined &&
      invite.templateIds.includes(sheet.templateId) &&
      sheet.campaignId === undefined,
  );
  const templateId = invite?.templateId ?? invite?.templateIds[0];
  const characterLimit = isSheetLimitReached(plan.data, 'character');

  const preview = useMutation<CampaignInvite, unknown, string>({
    mutationFn: (value) => client.previewCampaignInvite(code, value),
    onSuccess: (value) => {
      setPreviewInvite(value);
      setPasswordError(undefined);
    },
    onError: (error) => {
      setPasswordError(failureCopy(error));
    },
  });

  const join = useMutation<Campaign, unknown, { password?: string; sheetId?: string }>({
    mutationFn: ({ password: sentPassword, sheetId: sentSheet }) =>
      client.joinCampaign({ code, password: sentPassword, sheetId: sentSheet }),
    onMutate: () => {
      setJoinError(undefined);
    },
    onSuccess: (campaign) => {
      seedCampaign(queryClient, campaign);
      void queryClient.invalidateQueries({ queryKey: CAMPAIGNS_QUERY_KEY });
      navigate(`/campaigns/${encodeURIComponent(campaign.id)}`, {
        replace: true,
        state: { joinedName: campaign.name },
      });
    },
    onError: (error) => {
      setJoinError(failureCopy(error));
    },
  });

  useEffect(() => {
    if (invite?.alreadyMember !== true || autoJoined.current) {
      return;
    }
    autoJoined.current = true;
    join.mutate({ password: password === '' ? undefined : password });
  }, [invite, join, password]);

  const submitPassword = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (password.trim() === '' || preview.isPending) {
      return;
    }
    preview.mutate(password);
  };

  const submitJoin = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (join.isPending) {
      return;
    }
    join.mutate({
      password: password === '' ? undefined : password,
      sheetId: sheetId === '' ? undefined : sheetId,
    });
  };

  const retryMember = () => {
    autoJoined.current = false;
    join.mutate({ password: password === '' ? undefined : password });
  };

  if (invite !== undefined) {
    if (invite.alreadyMember) {
      return (
        <JoinCard title={invite.name}>
          {join.isPending ? (
            <LoadingState label={message('join.alreadyMember')} />
          ) : joinError === undefined ? (
            <LoadingState label={message('join.alreadyMember')} />
          ) : (
            <>
              <ErrorState message={joinError} />
              <div className="lg-join__actions">
                <Button
                  variant="primary"
                  size="md"
                  label={message('join.submit')}
                  onClick={retryMember}
                />
              </div>
            </>
          )}
        </JoinCard>
      );
    }

    return (
      <JoinCard title={invite.name} lead={message('join.invited')}>
        <p className="lg-join__meta">
          {invite.gmUsername === '' ? null : (
            <span>{messageWith('join.gm', { name: invite.gmUsername })}</span>
          )}
          <span>{invite.scopeName}</span>
          <span>{playerCount(invite.playerCount)}</span>
        </p>
        {joinError === undefined ? null : <ErrorState message={joinError} />}
        {eligibleSheets.length > 0 ? (
          <form className="lg-join__form" onSubmit={submitJoin}>
            <Select
              label={message('join.sheetLabel')}
              id="join-sheet"
              value={sheetId}
              hint={message('join.sheetHint')}
              onChange={setSheetId}
            >
              <option value="">{message('join.noSheet')}</option>
              {eligibleSheets.map((sheet) => (
                <option key={sheet.id} value={sheet.id}>
                  {sheet.name}
                </option>
              ))}
            </Select>
            <div className="lg-join__actions">
              <Button
                type="submit"
                variant="primary"
                size="md"
                fullWidth
                loading={join.isPending}
                label={join.isPending ? message('join.submitting') : message('join.submit')}
              />
            </div>
          </form>
        ) : (
          <>
            <Notice tone={characterLimit ? 'warning' : 'info'}>
              {characterLimit ? message('join.planLimit') : message('join.noEligible')}
            </Notice>
            {characterLimit ? (
              <Link className="lg-join__link" to="/plans">
                {message('campaigns.seePlans')}
              </Link>
            ) : null}
            <div className="lg-join__actions">
              <Button
                variant="primary"
                size="md"
                fullWidth
                label={message('join.createSheet')}
                disabled={join.isPending || templateId === undefined}
                onClick={() => setCreateOpen(true)}
              />
              <Button
                variant="secondary"
                size="md"
                fullWidth
                loading={join.isPending}
                label={join.isPending ? message('join.submitting') : message('join.joinWithout')}
                onClick={() => join.mutate({ password: password === '' ? undefined : password })}
              />
            </div>
          </>
        )}
        <p className="lg-join__footer">
          <Link className="lg-join__link" to="/campaigns">
            {message('campaign.back')}
          </Link>
        </p>
        {createOpen && templateId !== undefined ? (
          <CreateSheetDialog
            templateId={templateId}
            allowedKinds={['character']}
            onClose={() => setCreateOpen(false)}
            onCreated={async (sheet) => {
              await join.mutateAsync({
                password: password === '' ? undefined : password,
                sheetId: sheet.id,
              });
            }}
          />
        ) : null}
      </JoinCard>
    );
  }

  if (passwordRequired) {
    return (
      <JoinCard title={message('join.passwordTitle')} lead={message('join.passwordIntro')}>
        <form className="lg-join__form" onSubmit={submitPassword}>
          <Field
            label={message('join.passwordLabel')}
            id="join-password"
            name="password"
            type="password"
            autoComplete="off"
            value={password}
            hint={message('join.passwordHint')}
            error={passwordError}
            onChange={setPassword}
          />
          <div className="lg-join__actions">
            <Button
              type="submit"
              variant="primary"
              size="md"
              fullWidth
              loading={preview.isPending}
              label={preview.isPending ? message('join.submitting') : message('join.submit')}
              disabledReason={
                password.trim() === '' && !preview.isPending
                  ? message('join.submitNeedsPassword')
                  : undefined
              }
            />
          </div>
        </form>
      </JoinCard>
    );
  }

  if (invalidInvite) {
    return (
      <JoinCard title={message('join.invalidTitle')} lead={message('join.invalidBody')}>
        <ErrorState message={failureCopy(inviteQuery.error)} />
        <p className="lg-join__footer">
          <Link className="lg-join__link" to="/campaigns">
            {message('campaign.back')}
          </Link>
        </p>
      </JoinCard>
    );
  }

  if (inviteQuery.isError) {
    return (
      <JoinCard title={message('join.title')}>
        <ErrorState
          message={failureCopy(inviteQuery.error)}
          onRetry={() => void inviteQuery.refetch()}
        />
      </JoinCard>
    );
  }

  return (
    <JoinCard title={message('join.title')}>
      <LoadingState label={message('join.loading')} />
    </JoinCard>
  );
}
