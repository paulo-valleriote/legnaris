import { Link } from 'react-router-dom';
import { message } from '../l10n/strings';
import { PageHeader } from '../ui/page_header';
import { EmptyState } from '../ui/state';

export function NotFound() {
  return (
    <section data-testid="not-found">
      <PageHeader title={message('notFound.title')} />
      <EmptyState
        message={message('notFound.body')}
        action={
          <Link className="lg-shell__state-link" to="/">
            {message('notFound.action')}
          </Link>
        }
      />
    </section>
  );
}
