import { useEffect, useRef } from 'react';
import { message } from '../l10n/strings';
import { focusTrap } from '../ui/focus_trap';
import { parseMarkdown } from './privacy_markdown';
import type { MarkdownBlock } from './privacy_markdown';
import { policyMarkdown } from './privacy_policy';
import type { PolicyLocale } from './privacy_policy';
import { PrivacyArticle, currentPolicyLocale } from './privacy_screen';
import '../catalog/catalog.css';
import './privacy.css';

const TITLE_ID = 'privacy-dialog-title';

interface PrivacyDialogProps {
  readonly onClose: () => void;
  readonly locale?: PolicyLocale;
}

interface SplitPolicy {
  readonly title: string;
  readonly body: readonly MarkdownBlock[];
}

function splitTitle(blocks: readonly MarkdownBlock[]): SplitPolicy {
  const [first, ...rest] = blocks;
  if (first?.kind === 'heading' && first.level === 1) {
    return { title: first.text, body: rest };
  }
  return { title: message('privacy.title'), body: blocks };
}

export function PrivacyDialog({ onClose, locale }: PrivacyDialogProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const resolved = locale ?? currentPolicyLocale();
  const { title, body } = splitTitle(parseMarkdown(policyMarkdown(resolved)));

  useEffect(() => {
    const panel = panelRef.current;
    if (panel === null) {
      return undefined;
    }
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    (scrollRef.current ?? panel).focus();
    const trap = focusTrap(panel);
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onClose();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      trap.release();
      opener?.focus();
    };
  }, [onClose]);

  return (
    <div
      className="lg-overlay"
      role="presentation"
      data-testid="privacy-dialog-overlay"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose();
        }
      }}
    >
      <div
        ref={panelRef}
        className="lg-dialog lg-privacy-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={TITLE_ID}
        tabIndex={-1}
        lang={resolved}
      >
        <div className="lg-dialog__header lg-privacy-dialog__header">
          <h2 className="lg-dialog__title lg-privacy-dialog__title" id={TITLE_ID}>
            {title}
          </h2>
          <button
            type="button"
            className="lg-button lg-button--ghost lg-privacy-dialog__close"
            aria-label={message('privacy.close')}
            title={message('privacy.close')}
            onClick={onClose}
          >
            <svg
              className="lg-privacy-dialog__close-icon"
              viewBox="0 0 24 24"
              aria-hidden="true"
              focusable="false"
            >
              <path d="M6 6l12 12M18 6L6 18" />
            </svg>
          </button>
        </div>
        <div
          ref={scrollRef}
          className="lg-privacy-dialog__scroll"
          role="region"
          aria-labelledby={TITLE_ID}
          // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
          tabIndex={0}
          data-testid="privacy-dialog-body"
        >
          <PrivacyArticle
            blocks={body}
            headingLevelOffset={1}
            className="lg-privacy-dialog__body"
          />
        </div>
      </div>
    </div>
  );
}
