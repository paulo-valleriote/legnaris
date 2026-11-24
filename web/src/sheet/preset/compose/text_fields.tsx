import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetLabel,
  usePresetSchema,
  usePresetValue,
} from '../preset_context';
import { textValue } from '../values';
import type { PresetTextField, PresetTextFieldsPiece } from './types';
import './compose.css';

function TextField({ field }: { readonly field: PresetTextField }) {
  const label = usePresetLabel(field.path);
  const value = textValue(usePresetValue(field.path));
  const schema = usePresetSchema(field.path);
  const commit = usePresetCommit(field.path);

  return (
    <div className="lg-compose-text-field">
      <span className="lg-compose-text-field__label">{label}</span>
      <InlineEdit
        className="lg-compose-text-field__value"
        label={label}
        path={field.path}
        schema={schema}
        value={value}
        inputMode="text"
        onCommit={commit}
      />
    </div>
  );
}

export function TextFields({ piece }: { readonly piece: PresetTextFieldsPiece }) {
  if (piece.fields.length === 0) {
    return null;
  }

  return (
    <div className="lg-compose-text-fields" data-testid="compose-text-fields">
      {piece.fields.map((field) => (
        <TextField key={field.path} field={field} />
      ))}
    </div>
  );
}
