import { useId, useState } from 'react';
import { SheetFieldControl } from '../../../engine/field_control';
import { message } from '../../../l10n/strings';
import { ChevronRightIcon } from '../../../ui/icons';
import {
  usePresetCommit,
  usePresetContext,
  usePresetField,
  usePresetLabel,
  usePresetValue,
} from '../preset_context';
import './play_section.css';
import './rules.css';

const RULE_PATHS = [
  'rules.modifierBase',
  'rules.modifierStep',
  'rules.passiveBase',
  'rules.spellSaveBase',
] as const;

function RuleField({ path }: { readonly path: string }) {
  const field = usePresetField(path);
  const label = usePresetLabel(path);
  const value = usePresetValue(path);
  const commit = usePresetCommit(path);

  if (field === undefined) {
    return null;
  }

  return (
    <div className="lg-rules__field">
      <span className="lg-rules__label">{label}</span>
      <span className="lg-rules__control" data-edit-path={field.path}>
        <SheetFieldControl
          path={field.path}
          schema={field.schema}
          value={value}
          onChanged={commit}
        />
      </span>
    </div>
  );
}

export function FantasyTableRules() {
  const { fields } = usePresetContext();
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const present = RULE_PATHS.filter((path) => fields.has(path));

  if (present.length === 0) {
    return null;
  }

  return (
    <section className="lg-play-section lg-rules" data-testid="sheet-table-rules">
      <h2 className="lg-play-section__title">
        <button
          type="button"
          className="lg-rules__toggle"
          aria-expanded={open}
          aria-controls={panelId}
          onClick={() => setOpen((current) => !current)}
        >
          <span>{message('sheet.play.rules')}</span>
          <ChevronRightIcon
            className={open ? 'lg-rules__chevron lg-rules__chevron--open' : 'lg-rules__chevron'}
          />
        </button>
      </h2>
      <div id={panelId} className="lg-rules__fields" hidden={!open}>
        {present.map((path) => (
          <RuleField key={path} path={path} />
        ))}
      </div>
    </section>
  );
}
