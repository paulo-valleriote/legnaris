/**
 * The "Your sheet" destination (WAS-25 … WAS-29, R10).
 *
 * It resolves the signed-in person's single owner-scoped sheet through the
 * controller and renders exactly one situation: loading, the designed empty
 * line, the sheet with an Open action, or the mapped failure with a
 * single-request Retry. Deleting is guarded by one confirmation dialog that
 * states the exact consequence; its safe action carries focus, Escape and the
 * overlay barrier both cancel, and only confirm issues the `DELETE`.
 */

import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Sheet } from '../api/models/sheet';
import { message } from '../l10n/strings';
import { useApiClient, useSheetStore } from '../app/providers';
import type { YourSheetController, YourSheetSnapshot } from './your_sheet';
import { createYourSheet } from './your_sheet';
import './sheet.css';
import './your_sheet.css';

interface DeleteDialogProps {
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
}

/**
 * The one delete confirmation (WAS-28): a labelled, modal dialog stating the
 * exact consequence, with the safe Cancel action focused, Escape and the overlay
 * barrier cancelling, and the destructive action a real button.
 */
function DeleteDialog({ onCancel, onConfirm }: DeleteDialogProps) {
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    cancelRef.current?.focus();
  }, []);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onCancel();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onCancel]);

  return (
    <div
      className="lg-overlay"
      role="presentation"
      data-testid="your-sheet-delete-overlay"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onCancel();
        }
      }}
    >
      <div
        className="lg-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="your-sheet-delete-title"
        aria-describedby="your-sheet-delete-message"
        data-testid="your-sheet-delete-dialog"
      >
        <div className="lg-dialog__header">
          <h2 className="lg-dialog__title" id="your-sheet-delete-title">
            {message('sheet.ownDeleteTitle')}
          </h2>
        </div>
        <p className="lg-dialog__message" id="your-sheet-delete-message">
          {message('sheet.ownDeleteMessage')}
        </p>
        <div className="lg-dialog__actions">
          <button type="button" className="lg-button" ref={cancelRef} onClick={onCancel}>
            {message('sheet.ownCancel')}
          </button>
          <button type="button" className="lg-button lg-button--danger" onClick={onConfirm}>
            {message('sheet.ownDeleteConfirm')}
          </button>
        </div>
      </div>
    </div>
  );
}

interface YourSheetBodyProps {
  readonly controller: YourSheetController;
  readonly snapshot: YourSheetSnapshot;
  readonly onOpen: (sheet: Sheet) => void;
}

function YourSheetBody({ controller, snapshot, onOpen }: YourSheetBodyProps) {
  const [confirming, setConfirming] = useState(false);

  switch (snapshot.state.kind) {
    case 'loading':
      return <p className="lg-sheet__message">{message('sheet.ownLoading')}</p>;
    case 'empty':
      return (
        <p className="lg-sheet__message" data-testid="your-sheet-empty">
          {message('sheet.ownEmpty')}
        </p>
      );
    case 'failure':
      return (
        <div className="lg-sheet__failure">
          <p className="lg-sheet__message" data-testid="your-sheet-failure">
            {snapshot.state.message}
          </p>
          <button
            type="button"
            className="lg-button"
            onClick={() => {
              void controller.resolve();
            }}
          >
            {message('sheet.ownRetry')}
          </button>
        </div>
      );
    case 'sheet': {
      const { sheet } = snapshot.state;
      return (
        <>
          <div className="lg-yoursheet__row">
            <span className="lg-yoursheet__name" data-testid="your-sheet-name">
              {sheet.name}
            </span>
            <div className="lg-yoursheet__actions">
              <button
                type="button"
                className="lg-button lg-button--primary"
                onClick={() => onOpen(sheet)}
              >
                {message('sheet.ownOpen')}
              </button>
              <button
                type="button"
                className="lg-button lg-button--danger"
                disabled={snapshot.deleting}
                aria-busy={snapshot.deleting}
                onClick={() => setConfirming(true)}
              >
                {message('sheet.ownDelete')}
              </button>
            </div>
          </div>
          {snapshot.deleteError !== undefined && (
            <p className="lg-sheet__error" role="alert" data-testid="your-sheet-delete-error">
              {snapshot.deleteError}
            </p>
          )}
          {confirming && (
            <DeleteDialog
              onCancel={() => setConfirming(false)}
              onConfirm={() => {
                setConfirming(false);
                void controller.remove(sheet.id);
              }}
            />
          )}
        </>
      );
    }
  }
}

/** The session-only destination for the signed-in person's own sheet (WAS-25). */
export function YourSheetScreen() {
  const client = useApiClient();
  const store = useSheetStore();
  const navigate = useNavigate();
  const controller = useMemo(() => createYourSheet({ client, store }), [client, store]);
  const snapshot = useSyncExternalStore(controller.subscribe, controller.getSnapshot);

  useEffect(() => {
    void controller.resolve();
  }, [controller]);

  return (
    <section
      className="lg-sheet"
      aria-labelledby="your-sheet-title"
      data-testid="your-sheet-screen"
    >
      <header className="lg-sheet__header">
        <h1 className="lg-sheet__title" id="your-sheet-title">
          {message('sheet.ownTitle')}
        </h1>
      </header>
      <YourSheetBody
        controller={controller}
        snapshot={snapshot}
        onOpen={(sheet) => navigate(`/sheets/${encodeURIComponent(sheet.id)}`)}
      />
    </section>
  );
}
