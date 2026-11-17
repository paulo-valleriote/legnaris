import { useState } from 'react';
import type { FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { Campaign, CampaignMember, NpcLink, NpcKind } from '../api/models/campaign';
import type { MemberAction } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { Dialog } from '../ui/dialog';
import { PLAN_QUERY_KEY } from '../catalog/queries';
import { InvitePanel } from './invite_panel';
import { useOwnedCharacterSheets, useOwnedSheets } from './campaign_queries';
import {
  CAMPAIGNS_QUERY_KEY,
  campaignKey,
  npcLinksKey,
  seedCampaign,
  useCampaign,
  useCampaignSheets,
  useNpcLinks,
} from './campaign_queries';
import {
  canDeleteCampaign,
  canLinkCharacter,
  eligibleSheetIds,
  isDeleting,
  isSelf,
  memberActions,
  unlinkedNpcSheetIds,
} from './campaign_view';
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
        <span className="lg-campaign__name">
          {name}
          {isSelf(campaign, member) ? ` (${message('campaign.you')})` : ''}
        </span>
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
  campaign,
  onUnlink,
  disabled,
  pending,
  error,
  onSubmit,
}: {
  campaign: Campaign;
  onUnlink: () => void;
  disabled: boolean;
  pending: boolean;
  error: string | undefined;
  onSubmit: (sheetId: string) => void;
}) {
  const owned = useOwnedCharacterSheets(true);
  const [choice, setChoice] = useState('');
  const eligible = owned.data === undefined ? [] : eligibleSheetIds(owned.data, campaign);
  const selected = eligible.includes(choice) ? choice : (eligible[0] ?? '');
  const names = new Map((owned.data ?? []).map((sheet) => [sheet.id, sheet.name]));

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (selected !== '') {
      onSubmit(selected);
    }
  };

  return (
    <section className="lg-campaign__section" aria-labelledby={`campaign-link-${campaign.id}`}>
      <h2 className="lg-campaign__heading" id={`campaign-link-${campaign.id}`}>
        {message('campaign.linkHeading')}
      </h2>
      {owned.isError ? (
        <p className="lg-campaign__error" role="alert">
          {failureCopy(owned.error)}
        </p>
      ) : null}
      {owned.data !== undefined && eligible.length === 0 ? (
        <p className="lg-campaign__message">{message('campaign.noEligibleSheets')}</p>
      ) : null}
      <form className="lg-campaign__form" onSubmit={submit} noValidate>
        {eligible.length === 0 ? null : (
          <label className="lg-field" htmlFor="campaign-character-sheet">
            <span className="lg-field__label">{message('campaign.linkLabel')}</span>
            <select
              id="campaign-character-sheet"
              className="lg-campaign__select"
              value={selected}
              disabled={disabled || pending}
              onChange={(event) => setChoice(event.target.value)}
            >
              {eligible.map((id) => (
                <option key={id} value={id}>
                  {names.get(id) ?? id}
                </option>
              ))}
            </select>
          </label>
        )}
        {error ? (
          <p className="lg-campaign__error" role="alert">
            {error}
          </p>
        ) : null}
        <div className="lg-campaign__actions">
          <button
            type="submit"
            className="lg-button lg-button--primary lg-button--md"
            disabled={disabled || pending || selected === ''}
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

function npcKindLabel(kind: NpcKind): string {
  return kind === 'enemy' ? message('campaign.kindEnemy') : message('campaign.kindNpc');
}

function LinkNpcForm({
  campaign,
  disabled,
  pending,
  error,
  onSubmit,
}: {
  campaign: Campaign;
  disabled: boolean;
  pending: boolean;
  error: string | undefined;
  onSubmit: (sheetId: string) => void;
}) {
  const npcs = useOwnedSheets('npc', true);
  const enemies = useOwnedSheets('enemy', true);
  const links = useNpcLinks(campaign.id, true);
  const [choice, setChoice] = useState('');
  const owned = [...(npcs.data ?? []), ...(enemies.data ?? [])];
  const linkedSheetIds = (links.data ?? []).map((link) => link.sheetId);
  const eligible = unlinkedNpcSheetIds(owned, campaign, linkedSheetIds);
  const selected = eligible.includes(choice) ? choice : (eligible[0] ?? '');
  const names = new Map(owned.map((sheet) => [sheet.id, sheet.name]));
  const loadError = npcs.error ?? enemies.error ?? links.error;
  const loaded = npcs.data !== undefined && enemies.data !== undefined && links.data !== undefined;

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (selected !== '') {
      onSubmit(selected);
    }
  };

  return (
    <form className="lg-campaign__form" onSubmit={submit} noValidate>
      {loadError ? (
        <p className="lg-campaign__error" role="alert">
          {failureCopy(loadError)}
        </p>
      ) : null}
      {loaded && eligible.length === 0 ? (
        <p className="lg-campaign__message">{message('campaign.noEligibleNpcs')}</p>
      ) : null}
      {eligible.length === 0 ? null : (
        <label className="lg-field" htmlFor="campaign-npc-sheet">
          <span className="lg-field__label">{message('campaign.npcLinkLabel')}</span>
          <select
            id="campaign-npc-sheet"
            className="lg-campaign__select"
            value={selected}
            disabled={disabled || pending}
            onChange={(event) => setChoice(event.target.value)}
          >
            {eligible.map((id) => (
              <option key={id} value={id}>
                {names.get(id) ?? id}
              </option>
            ))}
          </select>
        </label>
      )}
      {error ? (
        <p className="lg-campaign__error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="lg-campaign__actions">
        <button
          type="submit"
          className="lg-button lg-button--primary lg-button--md"
          disabled={disabled || pending || selected === ''}
          aria-busy={pending ? true : undefined}
        >
          {message('campaign.npcLink')}
        </button>
      </div>
    </form>
  );
}

function NpcLinks({
  campaignId,
  mode,
  pending,
  onUnlink,
  onToggle,
}: {
  campaignId: string;
  mode: 'gm' | 'player';
  pending: boolean;
  onUnlink?: (link: NpcLink) => void;
  onToggle?: (link: NpcLink) => void;
}) {
  const links = useNpcLinks(campaignId, true);
  const gm = mode === 'gm';

  if (links.isError) {
    return (
      <p className="lg-campaign__error" role="alert">
        {failureCopy(links.error)}
      </p>
    );
  }
  if (links.data === undefined) {
    return null;
  }
  if (links.data.length === 0) {
    return (
      <p className="lg-campaign__message">
        {message(gm ? 'campaign.npcsEmpty' : 'campaign.playerNpcsEmpty')}
      </p>
    );
  }

  return (
    <ul className="lg-campaign__list" aria-label={message('campaign.npcsHeading')}>
      {links.data.map((link) => (
        <li className="lg-campaign__row" key={link.id}>
          <div className="lg-campaign__member">
            <Link
              className="lg-campaign__link"
              to={`/campaigns/${campaignId}/npcs/${link.sheetId}`}
              aria-label={messageWith('campaign.openNpc', { name: link.name })}
            >
              {link.name}
            </Link>
            <span className="lg-campaign__meta">
              {npcKindLabel(link.kind)} ·{' '}
              {messageWith('campaign.npcOwner', {
                name: link.ownerName === '' ? message('campaign.unnamed') : link.ownerName,
              })}
            </span>
          </div>
          {gm ? (
            <div className="lg-campaign__actions">
              <button
                type="button"
                className="lg-button lg-button--outline lg-button--sm"
                aria-pressed={link.revealed}
                aria-label={messageWith('campaign.revealNpc', { name: link.name })}
                disabled={pending}
                onClick={() => onToggle?.(link)}
              >
                {link.revealed
                  ? message('campaign.hideNpcAction')
                  : message('campaign.revealNpcAction')}
              </button>
              {link.isOwner ? (
                <button
                  type="button"
                  className="lg-button lg-button--outline lg-button--sm"
                  aria-label={messageWith('campaign.unlinkNpc', { name: link.name })}
                  disabled={pending}
                  onClick={() => onUnlink?.(link)}
                >
                  {message('campaign.unlinkAction')}
                </button>
              ) : null}
            </div>
          ) : null}
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
  const [npcError, setNpcError] = useState<string | undefined>(undefined);
  const [npcNotice, setNpcNotice] = useState<string | undefined>(undefined);

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

  const linkNpc = useMutation({
    mutationFn: (sheetId: string) => client.linkNpc(campaign.id, sheetId),
    onSuccess: (link) => {
      setNpcError(undefined);
      setNpcNotice(message(link.revealed ? 'campaign.npcLinkedRevealed' : 'campaign.npcLinked'));
      void queryClient.invalidateQueries({ queryKey: npcLinksKey(campaign.id) });
    },
    onError: (error) => {
      setNpcNotice(undefined);
      setNpcError(failureCopy(error));
      if (failureKind(error) === 'notFound') {
        void queryClient.invalidateQueries({ queryKey: npcLinksKey(campaign.id) });
      }
    },
  });

  const unlinkNpc = useMutation({
    mutationFn: (sheetId: string) => client.unlinkNpc(campaign.id, sheetId),
    onSuccess: () => {
      setNpcError(undefined);
      setNpcNotice(message('campaign.npcUnlinked'));
      void queryClient.invalidateQueries({ queryKey: npcLinksKey(campaign.id) });
    },
    onError: (error) => {
      void queryClient.invalidateQueries({ queryKey: npcLinksKey(campaign.id) });
      if (failureKind(error) === 'notFound') {
        setNpcError(undefined);
        setNpcNotice(undefined);
        return;
      }
      setNpcNotice(undefined);
      setNpcError(failureCopy(error));
    },
  });

  const setRevealed = useMutation({
    mutationFn: ({
      sheetId,
      currentlyRevealed,
    }: {
      sheetId: string;
      currentlyRevealed: boolean;
    }) =>
      currentlyRevealed
        ? client.hideNpc(campaign.id, sheetId)
        : client.revealNpc(campaign.id, sheetId),
    onSuccess: (link) => {
      setNpcError(undefined);
      setNpcNotice(
        message(link.revealed ? 'campaign.npcRevealedNotice' : 'campaign.npcHiddenNotice'),
      );
      void queryClient.invalidateQueries({ queryKey: npcLinksKey(campaign.id) });
    },
    onError: (error) => {
      setNpcNotice(undefined);
      setNpcError(failureCopy(error));
      if (failureKind(error) === 'notFound') {
        void queryClient.invalidateQueries({ queryKey: npcLinksKey(campaign.id) });
      }
    },
  });

  const busy =
    memberCommand.isPending ||
    leave.isPending ||
    remove.isPending ||
    linkNpc.isPending ||
    unlinkNpc.isPending ||
    setRevealed.isPending;
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

      {campaign.viewerRole === 'gm' ? <InvitePanel campaign={campaign} /> : null}

      {canLinkCharacter(campaign) ? (
        <>
          <LinkCharacterForm
            campaign={campaign}
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

      {campaign.viewerRole === 'gm' ? (
        <section className="lg-campaign__section" aria-labelledby="campaign-npcs-heading">
          <h2 className="lg-campaign__heading" id="campaign-npcs-heading">
            {message('campaign.npcsHeading')}
          </h2>
          <LinkNpcForm
            campaign={campaign}
            disabled={deleting}
            pending={linkNpc.isPending || unlinkNpc.isPending}
            error={npcError}
            onSubmit={(sheetId) => linkNpc.mutate(sheetId)}
          />
          {npcNotice ? (
            <p className="lg-campaign__message" role="status">
              {npcNotice}
            </p>
          ) : null}
          <NpcLinks
            campaignId={campaign.id}
            mode="gm"
            pending={unlinkNpc.isPending || setRevealed.isPending}
            onUnlink={(link) => unlinkNpc.mutate(link.sheetId)}
            onToggle={(link) => {
              setNpcError(undefined);
              setNpcNotice(undefined);
              setRevealed.mutate({ sheetId: link.sheetId, currentlyRevealed: link.revealed });
            }}
          />
        </section>
      ) : null}

      {campaign.viewerRole === 'player' ? (
        <section className="lg-campaign__section" aria-labelledby="campaign-revealed-heading">
          <h2 className="lg-campaign__heading" id="campaign-revealed-heading">
            {message('campaign.npcsHeading')}
          </h2>
          <NpcLinks campaignId={campaign.id} mode="player" pending={false} />
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
