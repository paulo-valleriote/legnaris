import { AuthStage } from './auth_stage';

interface AuthBusyProps {
  readonly testId: string;
  readonly label: string;
}

export function AuthBusy({ testId, label }: AuthBusyProps) {
  return (
    <div className="lg-auth">
      <AuthStage />
      <section className="lg-auth__panel">
        <div className="lg-auth__restoring" role="status" aria-busy="true" data-testid={testId}>
          <span className="lg-spinner" aria-hidden="true" />
          <span>{label}</span>
        </div>
      </section>
    </div>
  );
}
