import { useState } from 'react';
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
import { Field } from '../ui/field';
import { atOwnedLimit } from './campaign_view';
import { CAMPAIGNS_QUERY_KEY, seedCampaign, useCampaigns } from './campaign_queries';
import './campaign.css';

export function roleText(role: CampaignRole, owner: boolean): string {
  if (owner) {
    return message('campaigns.owner');
  }
  return role === 'gm' ? message('campaigns.roleGm') : message('campaigns.rolePlayer');
}

function CampaignList({ campaigns }: { campaigns: readonly CampaignSummary[] }) {
  return (
    <ul className="lg-campaign__list" aria-label={message('campaigns.listHeading')}>
      {campaigns.map((campaign) => (
        <li className="lg-campaign__row" key={campaign.id}>
          <Link className="lg-campaign__link" to={`/campaigns/${campaign.id}`}>
            {campaign.name}
          </Link>
          <span className="lg-campaign__meta">
            {roleText(campaign.viewerRole, campaign.viewerOwner)}
          </span>
        </li>
      ))}
    </ul>
  );
}

function CreateCampaignForm({
  templates,
  disabled,
  pending,
  error,
  onSubmit,
}: {
  templates: readonly TemplateSummary[];
  disabled: boolean;
  pending: boolean;
  error: string | undefined;
  onSubmit: (name: string, templateId: string) => void;
}) {
  const [name, setName] = useState('');
  const [templateChoice, setTemplateChoice] = useState('');
  const [nameError, setNameError] = useState<string | undefined>(undefined);
  const templateId = templateChoice || templates[0]?.id || '';

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
    <form className="lg-campaign__form" onSubmit={submit} noValidate>
      <Field
        label={message('campaigns.nameLabel')}
        name="campaign-name"
        value={name}
        disabled={disabled || pending}
        error={nameError}
        onChange={setName}
      />
      <label className="lg-field" htmlFor="campaign-template">
        <span className="lg-field__label">{message('campaigns.templateLabel')}</span>
        <select
          id="campaign-template"
          className="lg-campaign__select"
          value={templateId}
          disabled={disabled || pending}
          onChange={(event) => setTemplateChoice(event.target.value)}
        >
          {templates.map((template) => (
            <option key={template.id} value={template.id}>
              {template.name}
            </option>
          ))}
        </select>
      </label>
      {error ? (
        <p className="lg-campaign__error" role="alert">
          {error}
        </p>
      ) : null}
      <div>
        <button
          type="submit"
          className="lg-button lg-button--primary lg-button--md"
          disabled={disabled || pending || templateId === ''}
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

  const create = useMutation({
    mutationFn: (input: { name: string; templateId: string }) => client.createCampaign(input),
    onSuccess: (campaign) => {
      setCreateError(undefined);
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

  const owned = plan.data?.ownedCampaigns;
  const limitReached = owned !== undefined && atOwnedLimit(owned);

  let listing;
  if (campaigns.isPending) {
    listing = (
      <p className="lg-campaign__message" role="status">
        {message('campaigns.loading')}
      </p>
    );
  } else if (campaigns.isError) {
    listing = (
      <div className="lg-campaign__failure" role="alert">
        <p className="lg-campaign__error">
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
    listing = <p className="lg-campaign__message">{message('campaigns.empty')}</p>;
  } else {
    listing = <CampaignList campaigns={campaigns.data} />;
  }

  return (
    <div className="lg-campaign">
      <h1 className="lg-campaign__title">{message('campaigns.title')}</h1>
      {listing}
      <section className="lg-campaign__section" aria-labelledby="campaigns-create-heading">
        <h2 className="lg-campaign__heading" id="campaigns-create-heading">
          {message('campaigns.createHeading')}
        </h2>
        {owned === undefined ? null : (
          <p className="lg-campaign__meta">
            {messageWith('campaigns.ownedCount', { used: owned.used, limit: owned.limit })}
          </p>
        )}
        {limitReached ? (
          <p className="lg-campaign__message" role="status">
            {message('campaigns.limitReached')}
          </p>
        ) : null}
        {templates.data === undefined ? null : (
          <CreateCampaignForm
            templates={templates.data}
            disabled={limitReached}
            pending={create.isPending}
            error={createError}
            onSubmit={(name, templateId) => create.mutate({ name, templateId })}
          />
        )}
      </section>
    </div>
  );
}
