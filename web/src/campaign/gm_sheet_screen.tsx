import { useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { UseQueryResult } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { TemplateDetail } from '../api/models/template';
import type { Sheet } from '../api/models/sheet';
import { useApiClient } from '../app/providers';
import { templateDetailKey } from '../catalog/queries';
import { message } from '../l10n/strings';
import { Button } from '../ui/button';
import { PageHeader } from '../ui/page_header';
import { TemplateName } from '../catalog/template_name';
import { ReadOnlySheet } from '../sheet/read_only_sheet';
import { ErrorState, LoadingState } from '../ui/state';
import type { GmSheetSnapshot } from './campaign_queries';
import { npcLinksKey, useNpcSheet, useGmSheet } from './campaign_queries';
import './campaign.css';

function useSheetTemplate(templateId: string | undefined) {
  const client = useApiClient();

  return useQuery({
    queryKey: templateDetailKey(templateId ?? ''),
    queryFn: () => client.getTemplate(templateId ?? ''),
    enabled: templateId !== undefined,
    retry: false,
  });
}

function SheetBody({
  template,
  sheet,
}: {
  readonly template: UseQueryResult<TemplateDetail, Error>;
  readonly sheet: Sheet;
}) {
  if (template.isError) {
    return <ErrorState message={failureCopy(template.error)} />;
  }
  if (template.data === undefined) {
    return <LoadingState label={message('gm.loading')} />;
  }

  return <ReadOnlySheet sheet={sheet} template={template.data} />;
}

export function GmSheetScreen() {
  const { id = '', sheetId = '' } = useParams();
  const sheet = useGmSheet(id, sheetId);

  return <GmSheetPage campaignId={id} heading={message('gm.heading')} sheet={sheet} />;
}

export function GmNpcSheetScreen() {
  const { id = '', sheetId = '' } = useParams();
  const sheet = useNpcSheet(id, sheetId);
  const queryClient = useQueryClient();
  const unavailable = sheet.isError && failureKind(sheet.error) === 'notFound';

  useEffect(() => {
    if (unavailable) {
      void queryClient.invalidateQueries({ queryKey: npcLinksKey(id) });
    }
  }, [id, queryClient, unavailable]);

  return (
    <GmSheetPage
      campaignId={id}
      heading={message('gm.npcHeading')}
      sheet={sheet}
      unavailable={unavailable ? message('gm.npcUnavailable') : undefined}
    />
  );
}

interface GmSheetPageProps {
  readonly campaignId: string;
  readonly heading: string;
  readonly sheet: UseQueryResult<GmSheetSnapshot, Error>;
  readonly unavailable?: string | undefined;
}

function GmSheetPage({ campaignId, heading, sheet, unavailable }: GmSheetPageProps) {
  const templateId = sheet.data?.sheet.templateId;
  const template = useSheetTemplate(templateId);
  const templateLabel =
    templateId === undefined ? undefined : (
      <span>
        {message('gm.templatePrefix')}
        <TemplateName
          name={template.data?.name ?? templateId}
          notice={template.data?.gameSystem?.notice}
        />
      </span>
    );

  return (
    <div className="lg-campaign">
      <PageHeader
        title={unavailable === undefined ? (sheet.data?.sheet.name ?? heading) : heading}
        backTo={`/campaigns/${campaignId}`}
        backLabel={message('campaign.back')}
        meta={templateLabel === undefined ? undefined : <span>{templateLabel}</span>}
        actions={
          <Button
            variant="outline"
            size="sm"
            label={sheet.isFetching ? message('gm.refreshing') : message('gm.refresh')}
            loading={sheet.isFetching}
            onClick={() => void sheet.refetch()}
          />
        }
      />
      {sheet.isPending ? <LoadingState label={message('gm.loading')} /> : null}
      {unavailable ? (
        <ErrorState message={unavailable} />
      ) : sheet.isError ? (
        <ErrorState message={failureCopy(sheet.error)} />
      ) : null}
      {sheet.data === undefined || unavailable ? null : (
        <>
          <p className="lg-campaign__caption">{message('gm.caption')}</p>
          <SheetBody template={template} sheet={sheet.data.sheet} />
        </>
      )}
    </div>
  );
}
