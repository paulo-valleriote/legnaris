import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  useSyncExternalStore,
} from 'react';
import type { ReactNode } from 'react';
import type { Sheet } from '../../api/models/sheet';
import { valueAtPath } from '../../api/models/sheet';
import type { FieldSchema, ResetAction, TemplateDetail } from '../../api/models/template';
import type { RollMode } from '../../dice/roll';
import type { SheetField } from '../../engine/field_grouping';
import { buildSections, fieldDisplayName, lastSegment } from '../../engine/field_grouping';
import type { DiceContextValue } from '../dice_row';
import type { SheetEditStore } from '../sheet_edit';

export interface PresetResetBinding {
  readonly run: (action: ResetAction, requestId: string) => Promise<Record<string, unknown>>;
  readonly onApplied: (action: ResetAction, cascade: Record<string, unknown>) => void;
  readonly refresh: () => Promise<void>;
  readonly requestId?: () => string;
}

export interface PresetContextValue {
  readonly store: SheetEditStore;
  readonly template: TemplateDetail;
  readonly sheet: Sheet;
  readonly dice: DiceContextValue;
  readonly fields: ReadonlyMap<string, SheetField>;
  readonly reset: PresetResetBinding | undefined;
  readonly rollMode: RollMode;
  readonly setRollMode: (mode: RollMode) => void;
  readonly consumeRollMode: () => void;
}

const PresetContext = createContext<PresetContextValue | null>(null);

interface PresetProviderProps extends Omit<
  PresetContextValue,
  'fields' | 'reset' | 'rollMode' | 'setRollMode' | 'consumeRollMode'
> {
  readonly reset?: PresetResetBinding;
  readonly children: ReactNode;
}

export function PresetProvider({
  store,
  template,
  sheet,
  dice,
  reset,
  children,
}: PresetProviderProps) {
  const fields = useMemo(
    () =>
      new Map(
        buildSections(template.fields)
          .flatMap((section) => section.leaves)
          .map((field) => [field.path, field]),
      ),
    [template],
  );
  const [rollMode, setRollMode] = useState<RollMode>('normal');
  const consumeRollMode = useCallback(() => setRollMode('normal'), []);
  const value = useMemo(
    () => ({ store, template, sheet, dice, fields, reset, rollMode, setRollMode, consumeRollMode }),
    [store, template, sheet, dice, fields, reset, rollMode, consumeRollMode],
  );
  return <PresetContext.Provider value={value}>{children}</PresetContext.Provider>;
}

export function usePresetContext(): PresetContextValue {
  const value = useContext(PresetContext);
  if (value === null) {
    throw new Error('a sheet preset section must render inside its provider');
  }
  return value;
}

export function usePresetField(path: string): SheetField | undefined {
  return usePresetContext().fields.get(path);
}

export function usePresetSchema(path: string): FieldSchema | undefined {
  return usePresetField(path)?.schema;
}

export function usePresetLabel(path: string): string {
  const field = usePresetField(path);
  return fieldDisplayName(lastSegment(path), field?.schema);
}

export function usePresetValue(path: string): unknown {
  const { store } = usePresetContext();
  const subscribe = useCallback(
    (listener: () => void) => store.subscribePath(path, listener),
    [store, path],
  );
  return useSyncExternalStore(subscribe, () => valueAtPath(store.getState().data, path));
}

export function usePresetCommit(path: string): (value: unknown) => void {
  const { store } = usePresetContext();
  return useCallback((value: unknown) => store.setPath(path, value), [store, path]);
}
