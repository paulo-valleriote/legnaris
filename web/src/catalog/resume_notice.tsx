import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { useApiClient } from '../app/providers';
import { usePlanUsage } from './queries';
import { generationCopy, message } from '../l10n/strings';
import { Button } from '../ui/button';
import { Notice } from '../ui/notice';

export function ResumeNotice() {
  const client = useApiClient();
  const navigate = useNavigate();
  const plan = usePlanUsage();
  const activeId = plan.data?.activeGenerationId ?? null;

  const generation = useQuery({
    queryKey: ['generation', activeId],
    queryFn: () => client.getGeneration(activeId ?? ''),
    enabled: activeId !== null,
    retry: false,
    refetchOnWindowFocus: false,
  });

  if (activeId === null || generation.data === undefined) {
    return null;
  }
  const { templateId, kind } = generation.data;

  return (
    <Notice
      tone="info"
      live="polite"
      action={
        <Button
          variant="secondary"
          size="md"
          label={message('generate.resumeOpen')}
          onClick={() =>
            navigate(`/generate/${encodeURIComponent(templateId)}?kind=${kind}`, {
              state: { generationId: activeId },
            })
          }
        />
      }
    >
      {generationCopy('generation.resumeNotice', kind)}
    </Notice>
  );
}
