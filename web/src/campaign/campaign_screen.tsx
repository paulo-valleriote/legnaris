import { useState } from 'react';
import type { FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { Campaign, CampaignMember } from '../api/models/campaign';
import type { MemberAction } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { Dialog } from '../ui/dialog';
import { Field } from '../ui/field';
import { PLAN_QUERY_KEY } from '../catalog/queries';
import {
  CAMPAIGNS_QUERY_KEY,
  campaignKey,
  seedCampaign,
  useCampaign,
  useCampaignSheets,
} from './campaign_queries';
import { canDeleteCampaign, canLinkCharacter, isDeleting, memberActions } from './campaign_view';
import { roleText } from './campaigns_screen';
import { useApiClient } from '../app/providers';
import './campaign.css';

type Pending =
  { kind: 'remove'; member: CampaignMember } | { kind: 'leave' } | { kind: 'delete' } | null;

type MemberCommand = { userId: string; action: MemberAction | 'remove' };

function memberName(member: CampaignMember): string {
  return member.username === '' ? message('campaign.unnamed') : member.username;
}

function MemberRow({
  campaign,
  member,
  onAction,
}: {
  campaign: Campaign;
  member: CampaignMember;
  onAction: (action: MemberAction | 'remove') => void;
}) {
  const name = memberName(member);
  const actions = memberActions(campaign, member);
  const labels: Record<MemberAction | 'remove', string> = {
    promote: message('campaign.promote'),
    demote: message('campaign.demote'),
    remove: message('campaign.remove'),
  };

  return (
    <li className="lg-campaign__row">
      <div className="lg-campaign__member">
        <span className="lg-campaign__name">{name}</span>
        <span className="lg-campaign__meta">
          {roleText(member.role, member.owner)} ·{' '}
          {member.characterName === ''
            ? message('campaign.noCharacter')
            : messageWith('campaign.characterName', { name: member.characterName })}
        </span>
      </div>
      {actions.length === 0 ? null : (
        <div className="lg-campaign__actions">
          {actions.map((action) => (
            <button
              key={action}
              type="button"
              className={`lg-button lg-button--${action === 'remove' ? 'danger' : 'outline'} lg-button--sm`}
              aria-label={`${labels[action]} ${name}`}
              onClick={() => onAction(action)}
            >
              {labels[action]}
            </button>
          ))}
        </div>
      )}
    </li>
  );
}

function LinkCharacterForm({
  campaignId,
  onUnlink,
  disabled,
  pending,
  error,
  onSubmit,
}: {
  campaignId: string;
  onUnlink: () => void;
  disabled: boolean;
  pending: boolean;
  error: string | undefined;
  onSubmit: (sheetId: string) => void;
}) {
  const [sheetId, setSheetId] = useState('');
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (sheetId.trim() !== '') {
      onSubmit(sheetId.trim());
    }
  };

  return (
    <section className="lg-campaign__section" aria-labelledby={`campaign-link-${campaignId}`}>
      <h2 className="lg-campaign__heading" id={`campaign-link-${campaignId}`}>
        {message('campaign.linkHeading')}
      </h2>
      <form className="lg-campaign__form" onSubmit={submit} noValidate>
        <Field
          label={message('campaign.linkLabel')}
          name="character-sheet-id"
          value={sheetId}
          disabled={disabled || pending}
          onChange={setSheetId}
        />
        {error ? (
          <p className="lg-campaign__error" role="alert">
            {error}
          </p>
        ) : null}
        <div className="lg-campaign__actions">
          <button
            type="submit"
            className="lg-button lg-button--primary lg-button--md"
            disabled={disabled || pending || sheetId.trim() === ''}
            aria-busy={pending ? true : undefined}
          >
            {message('campaign.link')}
          </button>
          <button
            type="button"
            className="lg-button lg-button--outline lg-button--md"
            disabled={disabled || pending}
            onClick={onUnlink}
          >
            {message('campaign.unlink')}
          </button>
        </div>
      </form>
    </section>
  );
}

function LinkedSheets({ campaignId }: { campaignId: string }) {
  const sheets = useCampaignSheets(campaignId, true);

  if (sheets.isError) {
    return (
      <p className="lg-campaign__error" role="alert">
        {failureCopy(sheets.error)}
      </p>
    );
  }
  if (sheets.data === undefined) {
    return null;
  }
  if (sheets.data.length === 0) {
    return <p className="lg-campaign__message">{message('campaign.sheetsEmpty')}</p>;
  }

  return (
    <ul className="lg-campaign__list" aria-label={message('campaign.sheetsHeading')}>
      {sheets.data.map((sheet) => (
        <li className="lg-campaign__row" key={sheet.id}>
          <Link
            className="lg-campaign__link"
            to={`/campaigns/${campaignId}/sheets/${sheet.id}`}
            aria-label={messageWith('campaign.openSheet', { name: sheet.name })}
          >
            {sheet.name}
          </Link>
        </li>
      ))}
    </ul>
  );
}

function CampaignBody({ campaign }: { campaign: Campaign }) {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [pending, setPending] = useState<Pending>(null);
  const [notice, setNotice] = useState<string | undefined>(undefined);
  const [actionError, setActionError] = useState<string | undefined>(undefined);
  const [linkError, setLinkError] = useState<string | undefined>(undefined);

  const refreshCampaign = (next: Campaign) => {
    seedCampaign(queryClient, next);
    void queryClient.invalidateQueries({ queryKey: CAMPAIGNS_QUERY_KEY });
  };

  const memberCommand = useMutation({
    mutationFn: ({ userId, action }: MemberCommand) =>
      action === 'remove'
        ? client.removeMember(campaign.id, userId)
        : client.changeMemberRole(campaign.id, userId, action),
    onSuccess: (next) => {
      setPending(null);
      setActionError(undefined);
      refreshCampaign(next);
    },
    onError: (error) => {
      setPending(null);
      setActionError(failureCopy(error));
      if (failureKind(error) === 'campaignNotFound') {
        void queryClient.invalidateQueries({ queryKey: campaignKey(campaign.id) });
      }
    },
  });

  const leave = useMutation({
    mutationFn: () => client.leaveCampaign(campaign.id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: CAMPAIGNS_QUERY_KEY });
      void navigate('/campaigns');
    },
    onError: (error) => {
      setPending(null);
      setActionError(failureCopy(error));
    },
  });

  const remove = useMutation({
    mutationFn: () => client.deleteCampaign(campaign.id),
    onSuccess: async () => {
      setPending(null);
      await queryClient.invalidateQueries({ queryKey: CAMPAIGNS_QUERY_KEY });
      await queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
      void navigate('/campaigns');
    },
    onError: (error) => {
      setPending(null);
      setActionError(failureCopy(error));
    },
  });

  const link = useMutation({
    mutationFn: (sheetId: string) => client.linkCharacter(campaign.id, sheetId),
    onSuccess: (next) => {
      setLinkError(undefined);
      setNotice(message('campaign.linked'));
      refreshCampaign(next);
    },
    onError: (error) => {
      setNotice(undefined);
      setLinkError(failureCopy(error));
    },
  });

  const unlink = useMutation({
    mutationFn: () => client.unlinkCharacter(campaign.id),
    onSuccess: (next) => {
      setLinkError(undefined);
      setNotice(message('campaign.unlinked'));
      refreshCampaign(next);
    },
    onError: (error) => {
      setNotice(undefined);
      setLinkError(failureCopy(error));
    },
  });

  const busy = memberCommand.isPending || leave.isPending || remove.isPending;
  const deleting = isDeleting(campaign);

  const confirmPending = () => {
    if (pending?.kind === 'remove') {
      memberCommand.mutate({ userId: pending.member.userId, action: 'remove' });
    } else if (pending?.kind === 'leave') {
      leave.mutate();
    } else if (pending?.kind === 'delete') {
      remove.mutate();
    }
  };

  const closePending = () => {
    if (!busy) {
      setPending(null);
    }
  };

  const onMemberAction = (member: CampaignMember, action: MemberAction | 'remove') => {
    setActionError(undefined);
    if (action === 'remove') {
      setPending({ kind: 'remove', member });
    } else {
      memberCommand.mutate({ userId: member.userId, action });
    }
  };

  return (
    <>
      <p className="lg-campaign__back">
        <Link to="/campaigns">{message('campaign.back')}</Link>
      </p>
      <h1 className="lg-campaign__title">{campaign.name}</h1>
      <p className="lg-campaign__meta">{roleText(campaign.viewerRole, campaign.viewerOwner)}</p>
      {deleting ? (
        <p className="lg-campaign__message" role="status">
          {message('campaign.deleting')}
        </p>
      ) : null}
      {actionError ? (
        <p className="lg-campaign__error" role="alert">
          {actionError}
        </p>
      ) : null}

      <section className="lg-campaign__section" aria-labelledby="campaign-members-heading">
        <h2 className="lg-campaign__heading" id="campaign-members-heading">
          {message('campaign.membersHeading')}
        </h2>
        <ul className="lg-campaign__list">
          {campaign.members.map((member) => (
            <MemberRow
              key={member.userId}
              campaign={campaign}
              member={member}
              onAction={(action) => onMemberAction(member, action)}
            />
          ))}
        </ul>
      </section>

      {canLinkCharacter(campaign) ? (
        <>
          <LinkCharacterForm
            campaignId={campaign.id}
            onUnlink={() => unlink.mutate()}
            disabled={deleting}
            pending={link.isPending || unlink.isPending}
            error={linkError}
            onSubmit={(sheetId) => link.mutate(sheetId)}
          />
          {notice ? (
            <p className="lg-campaign__message" role="status">
              {notice}
            </p>
          ) : null}
        </>
      ) : null}

      {campaign.viewerRole === 'gm' ? (
        <section className="lg-campaign__section" aria-labelledby="campaign-sheets-heading">
          <h2 className="lg-campaign__heading" id="campaign-sheets-heading">
            {message('campaign.sheetsHeading')}
          </h2>
          <LinkedSheets campaignId={campaign.id} />
        </section>
      ) : null}

      <section className="lg-campaign__section" aria-labelledby="campaign-danger-heading">
        {canDeleteCampaign(campaign) ? (
          <>
            <h2 className="lg-campaign__heading" id="campaign-danger-heading">
              {message('campaign.deleteHeading')}
            </h2>
            <p className="lg-campaign__meta">{message('campaign.deleteNote')}</p>
            <button
              type="button"
              className="lg-button lg-button--danger lg-button--md"
              disabled={busy || deleting}
              onClick={() => setPending({ kind: 'delete' })}
            >
              {message('campaign.delete')}
            </button>
          </>
        ) : (
          <button
            type="button"
            className="lg-button lg-button--outline lg-button--md"
            disabled={busy || deleting}
            onClick={() => setPending({ kind: 'leave' })}
          >
            {message('campaign.leave')}
          </button>
        )}
      </section>

      <Dialog
        open={pending !== null}
        title={dialogTitle(pending)}
        description={dialogMessage(pending)}
        onClose={closePending}
      >
        <div className="lg-campaign__actions">
          <button
            type="button"
            className="lg-button lg-button--outline lg-button--md"
            onClick={closePending}
            disabled={busy}
          >
            {dialogKeep(pending)}
          </button>
          <button
            type="button"
            className="lg-button lg-button--danger lg-button--md"
            onClick={confirmPending}
            disabled={busy}
            aria-busy={busy ? true : undefined}
          >
            {dialogConfirm(pending)}
          </button>
        </div>
      </Dialog>
    </>
  );
}

function dialogTitle(pending: Pending): string {
  if (pending?.kind === 'remove') {
    return messageWith('campaign.removeTitle', { name: memberName(pending.member) });
  }
  if (pending?.kind === 'leave') {
    return message('campaign.leaveTitle');
  }
  return message('campaign.deleteTitle');
}

function dialogMessage(pending: Pending): string {
  if (pending?.kind === 'remove') {
    return message('campaign.removeMessage');
  }
  if (pending?.kind === 'leave') {
    return message('campaign.leaveMessage');
  }
  return message('campaign.deleteMessage');
}

function dialogKeep(pending: Pending): string {
  if (pending?.kind === 'remove') {
    return message('campaign.keepMember');
  }
  if (pending?.kind === 'leave') {
    return message('campaign.keepMe');
  }
  return message('campaign.keepCampaign');
}

function dialogConfirm(pending: Pending): string {
  if (pending?.kind === 'remove') {
    return message('campaign.removeConfirm');
  }
  if (pending?.kind === 'leave') {
    return message('campaign.leaveConfirm');
  }
  return message('campaign.deleteConfirm');
}

export function CampaignScreen() {
  const { id = '' } = useParams();
  const campaign = useCampaign(id);

  if (campaign.isPending) {
    return (
      <p className="lg-campaign__message" role="status">
        {message('campaign.loading')}
      </p>
    );
  }

  if (campaign.isError) {
    const notFound = failureKind(campaign.error) === 'campaignNotFound';
    return (
      <div className="lg-campaign__failure" role="alert">
        <p className="lg-campaign__error">
          {notFound ? message('campaign.notFound') : failureCopy(campaign.error)}
        </p>
        <Link to="/campaigns">{message('campaign.back')}</Link>
      </div>
    );
  }

  return (
    <div className="lg-campaign">
      <CampaignBody campaign={campaign.data} />
    </div>
  );
}
