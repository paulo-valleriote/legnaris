import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { useApiClient } from '../app/providers';
import { usePlanUsage } from './queries';
import { message } from '../l10n/strings';
import { Button } from '../ui/button';

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
  const templateId = generation.data.templateId;

  return (
    <div role="status" className="lg-catalog__notice">
      <p>{message('generate.resumeNotice')}</p>
      <Button
        variant="secondary"
        size="md"
        label={message('generate.resumeOpen')}
        onClick={() =>
          navigate(`/generate/${encodeURIComponent(templateId)}`, {
            state: { generationId: activeId },
          })
        }
      />
    </div>
  );
}
