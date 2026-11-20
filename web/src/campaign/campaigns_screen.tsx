import { useCallback, useEffect, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { AdSlot } from '../ads/ad_slot';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { CampaignRole, CampaignSummary } from '../api/models/campaign';
import type { TemplateSummary } from '../api/models/template';
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
import { atOwnedLimit } from './campaign_view';
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

function CampaignRow({
  campaign,
  templateName,
}: {
  campaign: CampaignSummary;
  templateName: string | undefined;
}) {
  const parts = [templateName, updatedText(campaign.updatedAt)].filter(
    (part): part is string => part !== undefined,
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

function CampaignList({
  campaigns,
  templateNames,
}: {
  campaigns: readonly CampaignSummary[];
  templateNames: ReadonlyMap<string, string>;
}) {
  return (
    <ul className="lg-campaigns__list" aria-label={message('campaigns.listHeading')}>
      {campaigns.map((campaign) => (
        <CampaignRow
          key={campaign.id}
          campaign={campaign}
          templateName={templateNames.get(campaign.templateId)}
        />
      ))}
    </ul>
  );
}

function CreateCampaignForm({
  templates,
  pending,
  error,
  onSubmit,
  onCancel,
}: {
  templates: readonly TemplateSummary[];
  pending: boolean;
  error: string | undefined;
  onSubmit: (name: string, templateId: string) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState('');
  const [templateChoice, setTemplateChoice] = useState('');
  const [nameError, setNameError] = useState<string | undefined>(undefined);
  const nameRef = useRef<HTMLInputElement>(null);
  const templateId = templateChoice || templates[0]?.id || '';

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
    setNameError(undefined);
    onSubmit(trimmed, templateId);
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
      {templates.length > 1 ? (
        <Select
          label={message('campaigns.templateLabel')}
          id="campaign-template"
          value={templateId}
          disabled={pending}
          options={templates.map((template) => ({ value: template.id, label: template.name }))}
          onChange={setTemplateChoice}
        />
      ) : templates.length === 1 ? (
        <dl className="lg-campaigns__template">
          <dt className="lg-campaigns__template-label">{message('campaigns.templateLabel')}</dt>
          <dd className="lg-campaigns__template-name" data-testid="campaigns-template">
            {templates[0].name}
          </dd>
        </dl>
      ) : null}
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
          disabled={templateId === ''}
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
    mutationFn: (input: { name: string; templateId: string }) => client.createCampaign(input),
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
  const templateNames = new Map(
    (templates.data ?? []).map((template) => [template.id, template.name]),
  );

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
    listing = (
      <CampaignList campaigns={orderCampaigns(campaigns.data)} templateNames={templateNames} />
    );
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
          templates={templates.data ?? []}
          pending={create.isPending}
          error={createError}
          onSubmit={(name, templateId) => create.mutate({ name, templateId })}
          onCancel={closeCreate}
        />
      </Dialog>
    </section>
  );
}
