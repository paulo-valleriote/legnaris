import { useMemo } from 'react';
import { newResetRequestID, ResetActions } from '../../reset_actions';
import { usePresetContext } from '../preset_context';
import type { PresetResetBarPiece } from './types';
import './compose.css';

export function ResetBar({ piece }: { readonly piece: PresetResetBarPiece }) {
  const { template, store, reset } = usePresetContext();
  const actions = useMemo(
    () =>
      piece.actions.flatMap((id) => {
        const action = template.actions?.find((entry) => entry.id === id);
        return action === undefined ? [] : [action];
      }),
    [piece.actions, template],
  );

  if (reset === undefined) {
    return null;
  }

  return (
    <ResetActions
      actions={actions}
      store={store}
      requestId={reset.requestId ?? newResetRequestID}
      run={reset.run}
      onApplied={reset.onApplied}
      refresh={reset.refresh}
    />
  );
}
