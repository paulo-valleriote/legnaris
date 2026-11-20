import { useEffect, useRef, useState } from 'react';
import { message, messageWith } from '../../../l10n/strings';
import { CloseIcon } from '../../../ui/icons';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetContext,
  usePresetField,
  usePresetLabel,
  usePresetValue,
} from '../preset_context';
import { numberValue } from '../values';
import { formatModifier } from './abilities_data';
import {
  ARMOR_ABILITIES,
  ARMOR_ABILITY_KEYS,
  ARMOR_CLASS_PATH,
  BASE_ARMOR_PATH,
  armorAbilityConfigPaths,
  armorAbilityFromValues,
  parseBaseArmor,
  type ArmorAbilityKey,
} from './armor_class';

const BREAKDOWN_ID = 'lg-ac-breakdown';

function ShieldShape({ className }: { readonly className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 86 96"
      preserveAspectRatio="xMidYMid meet"
      aria-hidden="true"
      focusable="false"
    >
      <path className="lg-ac__shape" d="M43 4 80 16v30c0 24-18 40-37 46C24 86 6 70 6 46V16Z" />
      <path className="lg-ac__shape-inner" d="M43 11 73 21v25c0 20-15 33-30 38" />
    </svg>
  );
}

function useArmorAbilityOptionLabel(key: ArmorAbilityKey): string {
  const field = usePresetField(ARMOR_ABILITIES[key].configPath);
  const option = field?.schema.optionLabel?.trim();
  return option === undefined || option === '' ? message(ARMOR_ABILITIES[key].shortLabel) : option;
}

function ArmorAbilitySegments({
  active,
  labels,
}: {
  readonly active: ArmorAbilityKey | undefined;
  readonly labels: Readonly<Record<ArmorAbilityKey, string>>;
}) {
  const { store } = usePresetContext();

  const choose = (key: ArmorAbilityKey) => {
    if (active === key) {
      return;
    }
    store.setExclusive(ARMOR_ABILITIES[key].configPath, armorAbilityConfigPaths());
  };

  return (
    <span
      className="lg-ac-breakdown__segments"
      role="group"
      aria-label={message('sheet.play.armorAbility')}
    >
      {ARMOR_ABILITY_KEYS.map((key) => (
        <button
          key={key}
          type="button"
          className="lg-ac-breakdown__segment"
          aria-label={labels[key]}
          aria-pressed={active === key}
          data-edit-path={ARMOR_ABILITIES[key].configPath}
          onClick={() => choose(key)}
        >
          {message(ARMOR_ABILITIES[key].shortLabel)}
        </button>
      ))}
    </span>
  );
}

function ArmorBreakdown({
  label,
  onClose,
}: {
  readonly label: string;
  readonly onClose: () => void;
}) {
  const panelRef = useRef<HTMLDivElement>(null);
  const baseArmor = numberValue(usePresetValue(BASE_ARMOR_PATH));
  const baseArmorSchema = usePresetField(BASE_ARMOR_PATH)?.schema;
  const total = numberValue(usePresetValue(ARMOR_CLASS_PATH));
  const active = armorAbilityFromValues({
    dex: numberValue(usePresetValue(ARMOR_ABILITIES.dex.configPath)),
    con: numberValue(usePresetValue(ARMOR_ABILITIES.con.configPath)),
  });
  const dexModifier = numberValue(usePresetValue(ARMOR_ABILITIES.dex.modifierPath));
  const conModifier = numberValue(usePresetValue(ARMOR_ABILITIES.con.modifierPath));
  const modifier = active === 'dex' ? dexModifier : active === 'con' ? conModifier : undefined;
  const commitBaseArmor = usePresetCommit(BASE_ARMOR_PATH);
  const baseLabel = usePresetLabel(BASE_ARMOR_PATH);
  const dexLabel = useArmorAbilityOptionLabel('dex');
  const conLabel = useArmorAbilityOptionLabel('con');
  const labels: Readonly<Record<ArmorAbilityKey, string>> = { dex: dexLabel, con: conLabel };

  useEffect(() => {
    panelRef.current?.focus();
  }, []);

  return (
    <div
      id={BREAKDOWN_ID}
      ref={panelRef}
      className="lg-ac-breakdown"
      data-testid="sheet-ac-breakdown"
      role="dialog"
      aria-label={label}
      tabIndex={-1}
    >
      <div className="lg-ac-breakdown__head">
        <h2 className="lg-ac-breakdown__title">{label}</h2>
        <button
          type="button"
          className="lg-ac-breakdown__close"
          aria-label={message('common.close')}
          onClick={onClose}
        >
          <CloseIcon />
        </button>
      </div>
      <dl className="lg-ac-breakdown__rows">
        <div className="lg-ac-breakdown__row">
          <dt className="lg-ac-breakdown__label">{baseLabel}</dt>
          <dd className="lg-ac-breakdown__value">
            <InlineEdit
              className="lg-ac-breakdown__base"
              label={baseLabel}
              path={BASE_ARMOR_PATH}
              schema={baseArmorSchema}
              value={baseArmor === undefined ? '' : String(baseArmor)}
              inputMode="numeric"
              onCommit={(text) => {
                const next = parseBaseArmor(text);
                if (next !== undefined) {
                  commitBaseArmor(next);
                }
              }}
            />
          </dd>
        </div>
        <div className="lg-ac-breakdown__row">
          <dt className="lg-ac-breakdown__label">{message('sheet.play.armorAbility')}</dt>
          <dd className="lg-ac-breakdown__value">
            <ArmorAbilitySegments active={active} labels={labels} />
            <span className="lg-ac-breakdown__modifier" data-testid="sheet-ac-modifier">
              {modifier === undefined ? message('sheet.play.noValue') : formatModifier(modifier)}
            </span>
          </dd>
        </div>
        <div className="lg-ac-breakdown__row lg-ac-breakdown__row--total">
          <dt className="lg-ac-breakdown__label">{label}</dt>
          <dd className="lg-ac-breakdown__value" data-testid="sheet-ac-total">
            {total === undefined ? message('sheet.play.noValue') : total}
          </dd>
        </div>
      </dl>
    </div>
  );
}

export function ArmorClassShield() {
  const total = numberValue(usePresetValue(ARMOR_CLASS_PATH));
  const label = usePresetLabel(ARMOR_CLASS_PATH);
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  const closeAndFocus = () => {
    setOpen(false);
    triggerRef.current?.focus();
  };

  useEffect(() => {
    if (!open) {
      return;
    }

    const onPointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && rootRef.current?.contains(target)) {
        return;
      }
      setOpen(false);
    };

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented) {
        return;
      }
      setOpen(false);
      triggerRef.current?.focus();
    };

    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const value = total === undefined ? message('sheet.play.noValue') : total;

  return (
    <div className="lg-ac" ref={rootRef} data-testid="sheet-ac">
      <button
        ref={triggerRef}
        type="button"
        className="lg-ac__shield"
        data-testid="sheet-ac-shield"
        aria-label={messageWith('sheet.play.armorClassValue', { count: value })}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? BREAKDOWN_ID : undefined}
        onClick={() => setOpen((current) => !current)}
      >
        <ShieldShape className="lg-ac__icon" />
        <span className="lg-ac__value" data-testid="sheet-ac-value">
          {value}
        </span>
        <span className="lg-ac__label">{message('sheet.play.armorClassShort')}</span>
      </button>
      {open && <ArmorBreakdown label={label} onClose={closeAndFocus} />}
    </div>
  );
}
