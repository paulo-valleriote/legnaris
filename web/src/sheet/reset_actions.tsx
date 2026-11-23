import { useCallback, useRef, useState, useSyncExternalStore } from 'react';
import { failureCopy, failureKind } from '../api/error_mapper';
import { isVersionConflict } from './save_retry';
import type { ResetAction } from '../api/models/template';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { isBusy, isPending, type SheetEditState, type SheetEditStore } from './sheet_edit';

export function newResetRequestID(): string {
  return crypto.randomUUID();
}

function blockedReason(state: SheetEditState): string | undefined {
  if (isPending(state)) {
    return message('sheet.reset.pending');
  }
  if (isBusy(state)) {
    return message('sheet.reset.busy');
  }
  return undefined;
}

export interface ResetActionsProps {
  readonly actions: readonly ResetAction[];
  readonly store: SheetEditStore;
  readonly requestId: () => string;
  readonly run: (action: ResetAction, requestId: string) => Promise<Record<string, unknown>>;
  readonly onApplied: (action: ResetAction, cascade: Record<string, unknown>) => void;
  readonly refresh: () => Promise<void>;
}

interface ResetFailure {
  readonly actionId: string;
  readonly copy: string;
  readonly conflict: boolean;
  readonly kind: string | undefined;
}

export function ResetActions({
  actions,
  store,
  requestId,
  run,
  onApplied,
  refresh,
}: ResetActionsProps) {
  const state = useSyncExternalStore(store.subscribe, store.getState);
  const [inFlight, setInFlight] = useState<string | undefined>(undefined);
  const [failure, setFailure] = useState<ResetFailure | undefined>(undefined);
  const [notice, setNotice] = useState('');
  const pendingAttempt = useRef<{ actionId: string; requestId: string } | undefined>(undefined);
  const working = useRef(false);
  const reason = blockedReason(state);

  const execute = useCallback(
    async (action: ResetAction, afterRefresh: boolean) => {
      if (working.current) {
        return;
      }
      working.current = true;
      const previous = pendingAttempt.current;
      const id =
        previous !== undefined && previous.actionId === action.id
          ? previous.requestId
          : requestId();
      pendingAttempt.current = { actionId: action.id, requestId: id };
      setInFlight(action.id);
      setFailure(undefined);
      setNotice('');
      try {
        if (afterRefresh) {
          await refresh();
        }
        const cascade = await run(action, id);
        pendingAttempt.current = undefined;
        onApplied(action, cascade);
        setNotice(messageWith('sheet.reset.applied', { label: action.label }));
      } catch (error) {
        setFailure({
          actionId: action.id,
          copy: isVersionConflict(error) ? message('sheet.reset.conflict') : failureCopy(error),
          conflict: isVersionConflict(error),
          kind: failureKind(error),
        });
      } finally {
        working.current = false;
        setInFlight(undefined);
      }
    },
    [onApplied, refresh, requestId, run],
  );

  const action = actions.find((candidate) => candidate.id === failure?.actionId);

  return (
    <div
      className="lg-sheet__reset"
      role="group"
      aria-label={message('sheet.reset.group')}
      data-testid="sheet-reset-actions"
    >
      <p
        className="lg-visually-hidden"
        role="status"
        aria-live="polite"
        data-testid="sheet-reset-notice"
      >
        {notice}
      </p>
      {actions.map((entry) => {
        const disabledReason = inFlight === undefined ? reason : message('sheet.reset.inFlight');
        return (
          <Button
            key={entry.id}
            variant="outline"
            size="md"
            label={entry.label}
            loading={inFlight === entry.id}
            disabledReason={disabledReason}
            onClick={() => {
              void execute(entry, false);
            }}
          />
        );
      })}
      {failure !== undefined && action !== undefined && (
        <div className="lg-sheet__reset-failure">
          <p className={resetErrorClass(failure.kind)} role="alert" data-testid="sheet-reset-error">
            {failure.copy}
          </p>
          <Button
            variant="ghost"
            size="sm"
            label={
              failure.conflict ? message('sheet.reset.retryAfterRefresh') : message('common.retry')
            }
            disabledReason={reason}
            onClick={() => {
              void execute(action, failure.conflict);
            }}
          />
        </div>
      )}
    </div>
  );
}

function resetErrorClass(kind: string | undefined): string {
  return kind === 'offline' ? 'lg-sheet__error lg-sheet__error--offline' : 'lg-sheet__error';
}
