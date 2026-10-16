/**
 * The sheet screen (WSC-05 … WSC-11): reads the `:id` route parameter, resolves
 * the open situation and renders it. A loaded sheet shows its name and the
 * template's field tree through the F4 engine; the loading, missing, retryable
 * and template situations each render their own mapped copy, and a template with
 * no fields shows the empty-fields line rather than an empty form.
 */

import { useParams } from 'react-router-dom';
import type { Sheet } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import { buildSections } from '../engine/field_grouping';
import { seedFromSheet } from '../engine/sheet_defaults';
import { SheetForm } from '../engine/sheet_form';
import { message } from '../l10n/strings';
import type { SheetOpenResult } from './sheet_open';
import { useSheetOpen } from './sheet_open';
import './sheet.css';

interface RetryActionProps {
  readonly copy: string;
  readonly onRetry: () => void;
}

function RetryAction({ copy, onRetry }: RetryActionProps) {
  return (
    <div className="lg-sheet__failure">
      <p className="lg-sheet__message">{copy}</p>
      <button type="button" className="lg-button" onClick={onRetry}>
        {message('sheet.retryOpen')}
      </button>
    </div>
  );
}

interface LoadedSheetProps {
  readonly sheet: Sheet;
  readonly template: TemplateDetail;
}

function LoadedSheet({ sheet, template }: LoadedSheetProps) {
  const sections = buildSections(template.fields);

  return (
    <>
      <header className="lg-sheet__header">
        <p className="lg-sheet__eyebrow">{message('sheet.headerTitle')}</p>
        <h1 className="lg-sheet__title" data-testid="sheet-name">
          {sheet.name}
        </h1>
      </header>
      {sections.length === 0 ? (
        <p className="lg-sheet__message" data-testid="sheet-empty-fields">
          {message('sheet.emptyFields')}
        </p>
      ) : (
        <SheetForm fields={template.fields} data={seedFromSheet(template.fields, sheet.data)} />
      )}
    </>
  );
}

function SheetBody({ result }: { readonly result: SheetOpenResult }) {
  switch (result.kind) {
    case 'loading':
      return <p className="lg-sheet__message">{message('sheet.openLoading')}</p>;
    case 'notFound':
      return (
        <p className="lg-sheet__message" data-testid="sheet-missing">
          {result.message}
        </p>
      );
    case 'failure':
      return <RetryAction copy={result.message} onRetry={result.retry} />;
    case 'templateLoading':
      return <p className="lg-sheet__message">{message('sheet.templateLoading')}</p>;
    case 'templateFailure':
      return <RetryAction copy={result.message} onRetry={result.retry} />;
    case 'loaded':
      return <LoadedSheet sheet={result.sheet} template={result.template} />;
  }
}

export function SheetScreen() {
  const params = useParams<{ id: string }>();
  const result = useSheetOpen(params.id ?? '');

  return (
    <section
      className="lg-sheet"
      aria-label={message('sheet.headerTitle')}
      data-testid="sheet-screen"
    >
      <SheetBody result={result} />
    </section>
  );
}
