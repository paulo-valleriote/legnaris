import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Sheet } from '../api/models/sheet';
import { message } from '../l10n/strings';
import { useApiClient, useSheetStore } from '../app/providers';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import { PageHeader } from '../ui/page_header';
import { EmptyState, ErrorState, LoadingState } from '../ui/state';
import type { YourSheetController, YourSheetSnapshot } from './your_sheet';
import { createYourSheet } from './your_sheet';
import './sheet.css';
import './your_sheet.css';

interface DeleteDialogProps {
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
}

function DeleteDialog({ onCancel, onConfirm }: DeleteDialogProps) {
  const safeRef = useRef<HTMLButtonElement>(null);

  return (
    <Dialog
      open
      tone="danger"
      title={message('sheet.ownDeleteTitle')}
      description={message('sheet.ownDeleteMessage')}
      onClose={onCancel}
      initialFocusRef={safeRef}
      actions={
        <>
          <Button
            ref={safeRef}
            variant="secondary"
            size="md"
            label={message('sheet.ownCancel')}
            onClick={onCancel}
          />
          <Button
            variant="danger"
            size="md"
            label={message('sheet.ownDeleteConfirm')}
            onClick={onConfirm}
          />
        </>
      }
    />
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
      return <LoadingState label={message('sheet.ownLoading')} />;
    case 'empty':
      return (
        <div data-testid="your-sheet-empty">
          <EmptyState message={message('sheet.ownEmpty')} />
        </div>
      );
    case 'failure':
      return (
        <div data-testid="your-sheet-failure">
          <ErrorState
            message={snapshot.state.message}
            onRetry={() => {
              void controller.resolve();
            }}
            retryLabel={message('common.retry')}
          />
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
              <Button
                variant="primary"
                size="md"
                label={message('sheet.ownOpen')}
                onClick={() => onOpen(sheet)}
              />
              <Button
                variant="danger"
                size="md"
                loading={snapshot.deleting}
                label={message('sheet.ownDelete')}
                onClick={() => setConfirming(true)}
              />
            </div>
          </div>
          {snapshot.deleteError !== undefined && (
            <div data-testid="your-sheet-delete-error">
              <ErrorState message={snapshot.deleteError} />
            </div>
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
      aria-label={message('sheet.ownTitle')}
      data-testid="your-sheet-screen"
    >
      <PageHeader title={message('sheet.ownTitle')} />
      <YourSheetBody
        controller={controller}
        snapshot={snapshot}
        onOpen={(sheet) => navigate(`/sheets/${encodeURIComponent(sheet.id)}`)}
      />
    </section>
  );
}
