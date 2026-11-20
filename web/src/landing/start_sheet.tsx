import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { useApiClient } from '../app/providers';
import { LANDING_PATH } from '../app/routes';
import { AuthBusy } from '../auth/auth_busy';
import { PLAN_QUERY_KEY, seedCreatedSheet } from '../catalog/queries';
import { message } from '../l10n/strings';

interface StartSheetProps {
  readonly templateId: string;
}

export function StartSheet({ templateId }: StartSheetProps) {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const started = useRef(false);

  useEffect(() => {
    if (started.current) {
      return;
    }
    started.current = true;
    void client
      .createSheet({
        templateId,
        kind: 'character',
        sheetName: message('landing.startSheetName'),
      })
      .then((sheet) => {
        seedCreatedSheet(queryClient, sheet);
        void queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
        navigate(`/sheets/${encodeURIComponent(sheet.id)}`, { replace: true });
      })
      .catch(() => {
        navigate(LANDING_PATH, { replace: true });
      });
  }, [client, templateId, queryClient, navigate]);

  return <AuthBusy testId="start-sheet" label={message('landing.startSheetPreparing')} />;
}
