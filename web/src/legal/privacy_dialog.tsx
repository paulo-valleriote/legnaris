import { useId, useRef } from 'react';
import { message } from '../l10n/strings';
import { Dialog } from '../ui/dialog';
import { policyDocument } from './privacy_policy';
import type { PolicyLocale } from './privacy_policy';
import { PrivacyArticle, currentPolicyLocale } from './privacy_screen';
import './privacy.css';

interface PrivacyDialogProps {
  readonly onClose: () => void;
  readonly locale?: PolicyLocale;
}

export function PrivacyDialog({ onClose, locale }: PrivacyDialogProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const resolved = locale ?? currentPolicyLocale();
  const { title, body } = policyDocument(resolved);

  return (
    <Dialog
      open
      title={title}
      titleId={titleId}
      closeLabel={message('privacy.close')}
      onClose={onClose}
      initialFocusRef={scrollRef}
      className="lg-privacy-dialog__panel"
    >
      <div
        ref={scrollRef}
        className="lg-privacy-dialog__scroll"
        role="region"
        aria-labelledby={titleId}
        lang={resolved}
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
        tabIndex={0}
        data-testid="privacy-dialog-body"
      >
        <PrivacyArticle blocks={body} headingLevelOffset={1} className="lg-privacy-dialog__body" />
      </div>
    </Dialog>
  );
}
