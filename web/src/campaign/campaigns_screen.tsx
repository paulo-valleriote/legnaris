import { useCallback, useEffect, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { AdSlot } from '../ads/ad_slot';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { CreateCampaignInput } from '../api/client';
import type { CampaignRole, CampaignSummary } from '../api/models/campaign';
import { useApiClient } from '../app/providers';
import { CATALOG_QUERY_KEY, PLAN_QUERY_KEY, usePlanUsage } from '../catalog/queries';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import { Field } from '../ui/field';
import { ChevronRightIcon } from '../ui/icons';
import { Notice } from '../ui/notice';
import { PageHeader } from '../ui/page_header';
import { Select } from '../ui/select';
import { EmptyState, ErrorState, LoadingState } from '../ui/state';
import type { CampaignScopeOption } from './campaign_view';
import { atOwnedLimit, campaignScopeOptions, createCampaignInput } from './campaign_view';
import { CAMPAIGNS_QUERY_KEY, seedCampaign, useCampaigns } from './campaign_queries';
import './campaigns.css';

export function roleText(role: CampaignRole, owner: boolean): string {
  if (owner) {
    return message('campaigns.owner');
  }
  return role === 'gm' ? message('campaigns.roleGm') : message('campaigns.rolePlayer');
}

export function updatedText(updatedAt: string): string | undefined {
  const date = new Date(updatedAt);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  const formatted = new Intl.DateTimeFormat('en', { dateStyle: 'medium' }).format(date);
  return messageWith('campaigns.updated', { date: formatted });
}

function campaignRank(campaign: CampaignSummary): number {
  if (campaign.viewerOwner) {
    return 0;
  }
  return campaign.viewerRole === 'gm' ? 1 : 2;
}

export function orderCampaigns(list: readonly CampaignSummary[]): CampaignSummary[] {
  return list
    .map((campaign, index) => ({ campaign, index }))
    .sort(
      (left, right) =>
        campaignRank(left.campaign) - campaignRank(right.campaign) || left.index - right.index,
    )
    .map((entry) => entry.campaign);
}

function CampaignRow({ campaign }: { campaign: CampaignSummary }) {
  const parts = [campaign.scopeName, updatedText(campaign.updatedAt)].filter(
    (part): part is string => part !== undefined && part !== '',
  );

  return (
    <li className="lg-campaigns__item">
      <Link className="lg-campaigns__entry" to={`/campaigns/${campaign.id}`}>
        <span className="lg-campaigns__text">
          <span className="lg-campaigns__name">{campaign.name}</span>
          {parts.length === 0 ? null : (
            <span className="lg-campaigns__meta">{parts.join(' · ')}</span>
          )}
        </span>
        <span className="lg-campaigns__trailing">
          <span className="lg-campaigns__role">
            {roleText(campaign.viewerRole, campaign.viewerOwner)}
          </span>
          <span className="lg-campaigns__open">
            <ChevronRightIcon />
          </span>
        </span>
      </Link>
    </li>
  );
}

function CampaignList({ campaigns }: { campaigns: readonly CampaignSummary[] }) {
  return (
    <ul className="lg-campaigns__list" aria-label={message('campaigns.listHeading')}>
      {campaigns.map((campaign) => (
        <CampaignRow key={campaign.id} campaign={campaign} />
      ))}
    </ul>
  );
}

function CreateCampaignForm({
  options,
  pending,
  error,
  onSubmit,
  onCancel,
}: {
  options: readonly CampaignScopeOption[];
  pending: boolean;
  error: string | undefined;
  onSubmit: (input: CreateCampaignInput) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState('');
  const [scopeChoice, setScopeChoice] = useState('');
  const [nameError, setNameError] = useState<string | undefined>(undefined);
  const nameRef = useRef<HTMLInputElement>(null);
  const selected = options.find((option) => option.key === scopeChoice) ?? options[0];

  useEffect(() => {
    nameRef.current?.focus();
  }, []);

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = name.trim();
    if (trimmed === '') {
      setNameError(message('campaigns.nameRequired'));
      return;
    }
    if (selected === undefined) {
      return;
    }
    setNameError(undefined);
    onSubmit(createCampaignInput(trimmed, selected));
  };

  return (
    <form className="lg-campaigns__form" onSubmit={submit} noValidate>
      <Field
        label={message('campaigns.nameLabel')}
        name="campaign-name"
        inputRef={nameRef}
        value={name}
        disabled={pending}
        error={nameError}
        onChange={setName}
      />
      {options.length > 1 ? (
        <Select
          label={message('campaigns.scopeLabel')}
          id="campaign-scope"
          value={selected?.key ?? ''}
          disabled={pending}
          options={options.map((option) => ({ value: option.key, label: option.label }))}
          onChange={setScopeChoice}
        />
      ) : selected === undefined ? null : (
        <dl className="lg-campaigns__template">
          <dt className="lg-campaigns__template-label">{message('campaigns.scopeLabel')}</dt>
          <dd className="lg-campaigns__template-name" data-testid="campaigns-template">
            {selected.label}
          </dd>
        </dl>
      )}
      {error ? <ErrorState message={error} /> : null}
      <div className="lg-campaigns__actions">
        <Button
          variant="outline"
          label={message('campaigns.cancel')}
          disabled={pending}
          onClick={onCancel}
        />
        <Button
          type="submit"
          label={pending ? message('campaigns.creating') : message('campaigns.create')}
          loading={pending}
          disabled={selected === undefined}
        />
      </div>
    </form>
  );
}

export function CampaignsScreen() {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const campaigns = useCampaigns();
  const plan = usePlanUsage();
  const templates = useQuery({
    queryKey: CATALOG_QUERY_KEY,
    queryFn: () => client.listTemplates(),
    retry: false,
  });
  const [createError, setCreateError] = useState<string | undefined>(undefined);
  const [createOpen, setCreateOpen] = useState(false);

  const create = useMutation({
    mutationFn: (input: CreateCampaignInput) => client.createCampaign(input),
    onSuccess: (campaign) => {
      setCreateError(undefined);
      setCreateOpen(false);
      seedCampaign(queryClient, campaign);
      void queryClient.invalidateQueries({ queryKey: CAMPAIGNS_QUERY_KEY });
      void queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
      void navigate(`/campaigns/${campaign.id}`);
    },
    onError: (error) => {
      setCreateError(failureCopy(error));
      if (failureKind(error) === 'planLimitReached') {
        void queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
      }
    },
  });

  const closeCreate = useCallback(() => {
    if (create.isPending) {
      return;
    }
    setCreateOpen(false);
  }, [create.isPending]);

  const openCreate = () => {
    setCreateError(undefined);
    setCreateOpen(true);
  };

  const owned = plan.data?.ownedCampaigns;
  const limitReached = owned !== undefined && atOwnedLimit(owned);
  const templatesMissing = templates.data === undefined;
  const templatesLoading = templatesMissing && templates.isPending;
  const templatesFailed = templatesMissing && templates.isError;
  const scopeOptions = campaignScopeOptions(templates.data ?? []);

  let listing;
  if (campaigns.isPending) {
    listing = <LoadingState label={message('campaigns.loading')} />;
  } else if (campaigns.isError) {
    listing = (
      <ErrorState
        message={`${message('campaigns.loadFailed')} ${failureCopy(campaigns.error)}`}
        onRetry={() => void campaigns.refetch()}
      />
    );
  } else if (campaigns.data.length === 0) {
    listing = <EmptyState message={message('campaigns.empty')} />;
  } else {
    listing = <CampaignList campaigns={orderCampaigns(campaigns.data)} />;
  }

  let createBlockedReason: string | undefined;
  if (templatesLoading) {
    createBlockedReason = message('campaigns.templatesLoading');
  } else if (templatesFailed) {
    createBlockedReason = message('campaigns.templatesFailed');
  }

  return (
    <section className="lg-campaigns">
      <PageHeader
        title={message('campaigns.title')}
        meta={
          owned === undefined ? undefined : (
            <span className="lg-campaigns__count" data-testid="campaigns-usage">
              {messageWith('campaigns.ownedCount', { used: owned.used, limit: owned.limit })}
            </span>
          )
        }
        actions={
          <Button
            label={message('campaigns.new')}
            disabled={limitReached}
            disabledReason={limitReached ? undefined : createBlockedReason}
            ariaDescribedBy={limitReached ? 'campaigns-limit' : undefined}
            onClick={openCreate}
          />
        }
      />
      {templatesFailed ? (
        <ErrorState
          message={message('campaigns.templatesFailed')}
          onRetry={() => void templates.refetch()}
        />
      ) : null}
      {limitReached ? (
        <Notice
          id="campaigns-limit"
          tone="warning"
          live="polite"
          action={
            <Link className="lg-campaigns__notice-link" to="/plans">
              {message('campaigns.seePlans')}
            </Link>
          }
        >
          {message('campaigns.limitReached')}
        </Notice>
      ) : null}
      {listing}
      <AdSlot surface="campaigns" />
      <Dialog
        open={createOpen}
        title={message('campaigns.create')}
        closeLabel={message('common.close')}
        onClose={closeCreate}
      >
        <CreateCampaignForm
          options={scopeOptions}
          pending={create.isPending}
          error={createError}
          onSubmit={(input) => create.mutate(input)}
          onCancel={closeCreate}
        />
      </Dialog>
    </section>
  );
}
