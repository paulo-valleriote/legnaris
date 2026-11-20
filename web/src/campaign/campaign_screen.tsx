import { useState } from 'react';
import type { FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { Campaign, CampaignMember, NpcLink, NpcKind } from '../api/models/campaign';
import type { MemberAction } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import { ChevronRightIcon } from '../ui/icons';
import { Notice } from '../ui/notice';
import { PageHeader } from '../ui/page_header';
import { Select } from '../ui/select';
import { EmptyState, ErrorState, LoadingState } from '../ui/state';
import { PLAN_QUERY_KEY } from '../catalog/queries';
import { INVITE_PANEL_ID, InvitePanel } from './invite_panel';
import { CampaignSettings, SETTINGS_PANEL_ID } from './campaign_settings';
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
  memberUsername,
  unlinkedNpcSheetIds,
} from './campaign_view';
import { roleText, updatedText } from './campaigns_screen';
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
            <Button
              key={action}
              variant={action === 'remove' ? 'danger' : 'outline'}
              size="sm"
              label={labels[action]}
              name={`${labels[action]} ${name}`}
              onClick={() => onAction(action)}
            />
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
      <SectionHeading id={`campaign-link-${campaign.id}`} label={message('campaign.linkHeading')} />
      {owned.isError ? <ErrorState message={failureCopy(owned.error)} /> : null}
      {owned.data !== undefined && eligible.length === 0 ? (
        <EmptyState message={message('campaign.noEligibleSheets')} />
      ) : null}
      <form className="lg-campaign__form" onSubmit={submit} noValidate>
        {eligible.length === 0 ? null : (
          <Select
            label={message('campaign.linkLabel')}
            id="campaign-character-sheet"
            value={selected}
            disabled={disabled || pending}
            options={eligible.map((id) => ({ value: id, label: names.get(id) ?? id }))}
            onChange={setChoice}
          />
        )}
        {error ? <ErrorState message={error} /> : null}
        <div className="lg-campaign__actions">
          <Button
            type="submit"
            label={message('campaign.link')}
            loading={pending}
            disabled={disabled || selected === ''}
          />
          <Button
            variant="outline"
            label={message('campaign.unlink')}
            disabled={disabled || pending}
            onClick={onUnlink}
          />
        </div>
      </form>
    </section>
  );
}

function SectionHeading({ id, label, count }: { id: string; label: string; count?: number }) {
  const shown = count !== undefined && count > 0;
  return (
    <h2 className="lg-campaign__heading" id={id} aria-label={shown ? `${label} (${count})` : label}>
      <span aria-hidden="true">{label}</span>
      {shown ? (
        <span className="lg-campaign__count" aria-hidden="true">
          {count}
        </span>
      ) : null}
    </h2>
  );
}

function ListLoading() {
  return <LoadingState label={message('campaign.listLoading')} />;
}

function LinkedSheets({ campaign }: { campaign: Campaign }) {
  const campaignId = campaign.id;
  const sheets = useCampaignSheets(campaignId, true);

  if (sheets.isError) {
    return <ErrorState message={failureCopy(sheets.error)} />;
  }
  if (sheets.data === undefined) {
    return <ListLoading />;
  }
  if (sheets.data.length === 0) {
    return <EmptyState message={message('campaign.sheetsEmpty')} />;
  }

  return (
    <ul className="lg-campaign__cards" aria-label={message('campaign.sheetsHeading')}>
      {sheets.data.map((sheet) => {
        const player = memberUsername(campaign, sheet.ownerId);
        const updated = updatedText(sheet.updatedAt);
        return (
          <li className="lg-campaign__card" key={sheet.id}>
            <Link
              className="lg-campaign__card-title"
              to={`/campaigns/${campaignId}/sheets/${sheet.id}`}
              aria-label={messageWith('campaign.openSheet', { name: sheet.name })}
            >
              <span className="lg-campaign__card-name">{sheet.name}</span>
              <span className="lg-campaign__card-arrow">
                <ChevronRightIcon />
              </span>
            </Link>
            {player === undefined ? null : (
              <span className="lg-campaign__meta">
                {messageWith('campaign.playedBy', { name: player })}
              </span>
            )}
            {updated === undefined ? null : (
              <span className="lg-campaign__card-foot">{updated}</span>
            )}
          </li>
        );
      })}
    </ul>
  );
}

function npcKindLabel(kind: NpcKind): string {
  return kind === 'enemy' ? message('kind.enemy') : message('kind.npc');
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
    <form className="lg-campaign__form lg-campaign__form--inline" onSubmit={submit} noValidate>
      {loadError ? <ErrorState message={failureCopy(loadError)} /> : null}
      {loaded && eligible.length === 0 ? (
        <EmptyState message={message('campaign.noEligibleNpcs')} />
      ) : null}
      {eligible.length === 0 ? null : (
        <Select
          label={message('campaign.npcLinkLabel')}
          id="campaign-npc-sheet"
          value={selected}
          disabled={disabled || pending}
          options={eligible.map((id) => ({ value: id, label: names.get(id) ?? id }))}
          onChange={setChoice}
        />
      )}
      {error ? <ErrorState message={error} /> : null}
      <div className="lg-campaign__actions">
        {eligible.length === 0 ? null : (
          <Button
            type="submit"
            label={message('campaign.npcLink')}
            loading={pending}
            disabled={disabled || selected === ''}
          />
        )}
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
    return <ErrorState message={failureCopy(links.error)} />;
  }
  if (links.data === undefined) {
    return <ListLoading />;
  }
  if (links.data.length === 0) {
    return <EmptyState message={message(gm ? 'campaign.npcsEmpty' : 'campaign.playerNpcsEmpty')} />;
  }

  return (
    <ul className="lg-campaign__cards" aria-label={message('campaign.npcsHeading')}>
      {links.data.map((link) => (
        <li className="lg-campaign__card" key={link.id}>
          <Link
            className="lg-campaign__card-title"
            to={`/campaigns/${campaignId}/npcs/${link.sheetId}`}
            aria-label={messageWith('campaign.openNpc', { name: link.name })}
          >
            <span className="lg-campaign__card-name">{link.name}</span>
            <span className="lg-campaign__card-arrow">
              <ChevronRightIcon />
            </span>
          </Link>
          <span className="lg-campaign__meta">
            {npcKindLabel(link.kind)} ·{' '}
            {messageWith('campaign.npcOwner', {
              name: link.ownerName === '' ? message('campaign.unnamed') : link.ownerName,
            })}
          </span>
          {gm ? (
            <span
              className={`lg-campaign__visibility${link.revealed ? ' lg-campaign__visibility--on' : ''}`}
            >
              {message(link.revealed ? 'campaign.npcVisible' : 'campaign.npcHidden')}
            </span>
          ) : null}
          {gm ? (
            <div className="lg-campaign__card-actions">
              <Button
                variant="outline"
                size="sm"
                label={
                  link.revealed
                    ? message('campaign.hideNpcAction')
                    : message('campaign.revealNpcAction')
                }
                name={messageWith('campaign.revealNpc', { name: link.name })}
                ariaPressed={link.revealed}
                disabled={pending}
                onClick={() => onToggle?.(link)}
              />
              {link.isOwner ? (
                <Button
                  variant="ghost"
                  size="sm"
                  label={message('campaign.unlinkAction')}
                  name={messageWith('campaign.unlinkNpc', { name: link.name })}
                  disabled={pending}
                  onClick={() => onUnlink?.(link)}
                />
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
  const [inviteOpen, setInviteOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);

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
  const gm = campaign.viewerRole === 'gm';
  const sheetCount = useCampaignSheets(campaign.id, gm).data?.length;
  const npcCount = useNpcLinks(campaign.id, true).data?.length;

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
      <PageHeader
        title={campaign.name}
        backTo="/campaigns"
        backLabel={message('campaign.back')}
        meta={
          <span className="lg-campaigns__role">
            {roleText(campaign.viewerRole, campaign.viewerOwner)}
          </span>
        }
        actions={
          campaign.viewerRole === 'gm' || canDeleteCampaign(campaign) ? (
            <div className="lg-campaign__toolbar">
              {campaign.viewerRole === 'gm' ? (
                <Button
                  variant="ghost"
                  size="md"
                  label={message('invite.heading')}
                  ariaExpanded={inviteOpen}
                  ariaControls={INVITE_PANEL_ID}
                  onClick={() => setInviteOpen((open) => !open)}
                />
              ) : null}
              {canDeleteCampaign(campaign) ? (
                <Button
                  variant="ghost"
                  size="md"
                  label={message('campaign.settingsHeading')}
                  ariaExpanded={settingsOpen}
                  ariaControls={SETTINGS_PANEL_ID}
                  onClick={() => setSettingsOpen((open) => !open)}
                />
              ) : null}
            </div>
          ) : undefined
        }
      />

      {deleting ? (
        <Notice tone="warning" live="polite">
          {message('campaign.deleting')}
        </Notice>
      ) : null}
      {actionError ? <ErrorState message={actionError} /> : null}

      {campaign.viewerRole === 'gm' && inviteOpen ? <InvitePanel campaign={campaign} /> : null}

      {canDeleteCampaign(campaign) && settingsOpen ? (
        <CampaignSettings
          campaign={campaign}
          busy={busy}
          deleting={deleting}
          onDelete={() => setPending({ kind: 'delete' })}
        />
      ) : null}

      <div className="lg-campaign__layout">
        <div className="lg-campaign__main">
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
                <Notice tone="success" live="polite">
                  {notice}
                </Notice>
              ) : null}
            </>
          ) : null}

          {gm ? (
            <section className="lg-campaign__section" aria-labelledby="campaign-sheets-heading">
              <SectionHeading
                id="campaign-sheets-heading"
                label={message('campaign.sheetsHeading')}
                count={sheetCount}
              />
              <LinkedSheets campaign={campaign} />
            </section>
          ) : null}

          {gm ? (
            <section className="lg-campaign__section" aria-labelledby="campaign-npcs-heading">
              <SectionHeading
                id="campaign-npcs-heading"
                label={message('campaign.npcsHeading')}
                count={npcCount}
              />
              <LinkNpcForm
                campaign={campaign}
                disabled={deleting}
                pending={linkNpc.isPending || unlinkNpc.isPending}
                error={npcError}
                onSubmit={(sheetId) => linkNpc.mutate(sheetId)}
              />
              {npcNotice ? (
                <Notice tone="success" live="polite">
                  {npcNotice}
                </Notice>
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
              <SectionHeading
                id="campaign-revealed-heading"
                label={message('campaign.npcsHeading')}
                count={npcCount}
              />
              <NpcLinks campaignId={campaign.id} mode="player" pending={false} />
            </section>
          ) : null}
        </div>

        <section
          className="lg-campaign__section lg-campaign__members"
          aria-labelledby="campaign-members-heading"
        >
          <SectionHeading
            id="campaign-members-heading"
            label={message('campaign.membersHeading')}
            count={campaign.members.length}
          />
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
          {canDeleteCampaign(campaign) ? null : (
            <div className="lg-campaign__leave">
              <Button
                variant="outline"
                label={message('campaign.leave')}
                disabled={busy || deleting}
                onClick={() => setPending({ kind: 'leave' })}
              />
            </div>
          )}
        </section>
      </div>

      <Dialog
        open={pending !== null}
        title={dialogTitle(pending)}
        description={dialogMessage(pending)}
        closeLabel={message('common.close')}
        onClose={closePending}
        actions={
          <>
            <Button
              variant="outline"
              label={dialogKeep(pending)}
              disabled={busy}
              onClick={closePending}
            />
            <Button
              variant="danger"
              label={dialogConfirm(pending)}
              loading={busy}
              onClick={confirmPending}
            />
          </>
        }
      />
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
    return <LoadingState label={message('campaign.loading')} />;
  }

  if (campaign.isError) {
    const notFound = failureKind(campaign.error) === 'campaignNotFound';
    return (
      <div className="lg-campaign__failure">
        <ErrorState
          message={notFound ? message('campaign.notFound') : failureCopy(campaign.error)}
        />
        <Link className="lg-campaign__link" to="/campaigns">
          {message('campaign.back')}
        </Link>
      </div>
    );
  }

  return (
    <div className="lg-campaign">
      <CampaignBody campaign={campaign.data} />
    </div>
  );
}
