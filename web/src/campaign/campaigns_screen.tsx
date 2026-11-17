import { useCallback, useEffect, useState } from 'react';
import type { FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { CampaignRole, CampaignSummary } from '../api/models/campaign';
import type { TemplateSummary } from '../api/models/template';
import { useApiClient } from '../app/providers';
import { CATALOG_QUERY_KEY, PLAN_QUERY_KEY, usePlanUsage } from '../catalog/queries';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import { Field } from '../ui/field';
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

function ChevronIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M5 12h14" />
      <path d="m13 6 6 6-6 6" />
    </svg>
  );
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
            <ChevronIcon />
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
  const templateId = templateChoice || templates[0]?.id || '';

  useEffect(() => {
    document.getElementById('campaign-name')?.focus();
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
        value={name}
        disabled={pending}
        error={nameError}
        onChange={setName}
      />
      {templates.length > 1 ? (
        <label className="lg-field" htmlFor="campaign-template">
          <span className="lg-field__label">{message('campaigns.templateLabel')}</span>
          <select
            id="campaign-template"
            className="lg-campaigns__select"
            value={templateId}
            disabled={pending}
            onChange={(event) => setTemplateChoice(event.target.value)}
          >
            {templates.map((template) => (
              <option key={template.id} value={template.id}>
                {template.name}
              </option>
            ))}
          </select>
        </label>
      ) : templates.length === 1 ? (
        <div className="lg-field">
          <span className="lg-field__label">{message('campaigns.templateLabel')}</span>
          <p className="lg-campaigns__template" data-testid="campaigns-template">
            {templates[0].name}
          </p>
        </div>
      ) : null}
      {error ? (
        <p className="lg-campaigns__error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="lg-campaigns__actions">
        <button
          type="button"
          className="lg-button lg-button--outline lg-button--md"
          disabled={pending}
          onClick={onCancel}
        >
          {message('campaigns.cancel')}
        </button>
        <button
          type="submit"
          className="lg-button lg-button--primary lg-button--md"
          disabled={pending || templateId === ''}
          aria-busy={pending ? true : undefined}
        >
          {pending ? message('campaigns.creating') : message('campaigns.create')}
        </button>
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
  const templateNames = new Map(
    (templates.data ?? []).map((template) => [template.id, template.name]),
  );

  let listing;
  if (campaigns.isPending) {
    listing = (
      <p className="lg-campaigns__message" role="status">
        {message('campaigns.loading')}
      </p>
    );
  } else if (campaigns.isError) {
    listing = (
      <div className="lg-campaigns__failure" role="alert">
        <p className="lg-campaigns__error">
          {message('campaigns.loadFailed')} {failureCopy(campaigns.error)}
        </p>
        <Button
          label={message('campaigns.retry')}
          variant="outline"
          onClick={() => void campaigns.refetch()}
        />
      </div>
    );
  } else if (campaigns.data.length === 0) {
    listing = <p className="lg-campaigns__empty">{message('campaigns.empty')}</p>;
  } else {
    listing = (
      <CampaignList campaigns={orderCampaigns(campaigns.data)} templateNames={templateNames} />
    );
  }

  return (
    <section className="lg-campaigns" aria-labelledby="campaigns-title">
      <header className="lg-campaigns__head">
        <div className="lg-campaigns__head-text">
          <h1 className="lg-campaigns__title" id="campaigns-title">
            {message('campaigns.title')}
          </h1>
          {owned === undefined ? null : (
            <p className="lg-campaigns__usage" data-testid="campaigns-usage">
              {messageWith('campaigns.ownedCount', { used: owned.used, limit: owned.limit })}
            </p>
          )}
        </div>
        {templates.data === undefined ? null : (
          <button
            type="button"
            className="lg-button lg-button--primary lg-button--md"
            disabled={limitReached}
            aria-describedby={limitReached ? 'campaigns-limit' : undefined}
            onClick={openCreate}
          >
            {message('campaigns.new')}
          </button>
        )}
      </header>
      {limitReached ? (
        <div className="lg-campaigns__notice" id="campaigns-limit" role="status">
          <p className="lg-campaigns__notice-text">{message('campaigns.limitReached')}</p>
          <Link className="lg-campaigns__notice-link" to="/plans">
            {message('campaigns.seePlans')}
          </Link>
        </div>
      ) : null}
      {listing}
      <Dialog open={createOpen} title={message('campaigns.create')} onClose={closeCreate}>
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
