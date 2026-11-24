import { message } from '../l10n/strings';
import { PageHeader } from '../ui/page_header';
import './privacy.css';

export function SrdScreen() {
  return (
    <main className="lg-privacy">
      <div className="lg-privacy__inner">
        <PageHeader title={message('srd.title')} backTo="/" backLabel={message('legal.backHome')} />
        <article className="lg-privacy__body">
          <p>{message('srd.attribution')}</p>
          <p>{message('srd.adaptation')}</p>
        </article>
      </div>
    </main>
  );
}
