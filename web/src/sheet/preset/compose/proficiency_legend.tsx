import { message } from '../../../l10n/strings';
import './compose.css';

export interface ProficiencyLegendProps {
  readonly testId: string;
  readonly expertise?: boolean;
}

function LegendDot() {
  return <span className="lg-compose-legend__dot" aria-hidden="true" />;
}

export function ProficiencyLegend({ testId, expertise = false }: ProficiencyLegendProps) {
  return (
    <p className="lg-compose-legend" data-testid={testId}>
      <span className="lg-compose-legend__entry">
        <LegendDot />
        {message('sheet.play.proficiencyProficient')}
      </span>
      {expertise ? (
        <span className="lg-compose-legend__entry">
          <span className="lg-compose-legend__marks" aria-hidden="true">
            <LegendDot />
            <LegendDot />
          </span>
          {message('sheet.play.proficiencyExpertise')}
        </span>
      ) : null}
    </p>
  );
}
