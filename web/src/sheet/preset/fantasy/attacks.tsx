import { useCallback, useMemo, useState, useSyncExternalStore } from 'react';
import { valueAtPath } from '../../../api/models/sheet';
import { groupDisplayName } from '../../../engine/field_grouping';
import { message, messageWith } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { PencilIcon } from '../../../ui/icons';
import { DieIcon } from '../../die_icon';
import { useExpressionRoll } from '../../dice_row';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetContext,
  usePresetLabel,
  usePresetSchema,
  usePresetValue,
} from '../preset_context';
import { RollableValue } from '../rollable_value';
import { numberValue, textValue } from '../values';
import { formatModifier } from './abilities_data';
import {
  ATTACK_SLOTS,
  attackVisibility,
  parseAttackBonus,
  parseAttackDamage,
  type AttackSlot,
} from './attacks_data';
import './attacks.css';

function AttackCard({ slot }: { readonly slot: AttackSlot }) {
  const { template } = usePresetContext();
  const name = textValue(usePresetValue(slot.namePath));
  const bonus = numberValue(usePresetValue(slot.bonusPath));
  const damage = textValue(usePresetValue(slot.damagePath));
  const notes = textValue(usePresetValue(slot.notesPath));
  const nameLabel = usePresetLabel(slot.namePath);
  const bonusLabel = usePresetLabel(slot.bonusPath);
  const damageLabel = usePresetLabel(slot.damagePath);
  const notesLabel = usePresetLabel(slot.notesPath);
  const nameSchema = usePresetSchema(slot.namePath);
  const bonusSchema = usePresetSchema(slot.bonusPath);
  const damageSchema = usePresetSchema(slot.damagePath);
  const notesSchema = usePresetSchema(slot.notesPath);
  const commitName = usePresetCommit(slot.namePath);
  const commitBonus = usePresetCommit(slot.bonusPath);
  const commitDamage = usePresetCommit(slot.damagePath);
  const commitNotes = usePresetCommit(slot.notesPath);
  const damageExpression = useMemo(() => parseAttackDamage(damage), [damage]);
  const rollDamage = useExpressionRoll(slot.rowKey, damageLabel);
  const attacksNode = template.fields.attacks;
  const slotKey = `attack${slot.slot}`;
  const slotNode = attacksNode?.kind === 'group' ? attacksNode.children[slotKey] : undefined;
  const fallbackName = groupDisplayName(
    slotKey,
    slotNode?.kind === 'group' ? slotNode.label : undefined,
  );
  const attackName = name.trim() === '' ? fallbackName : name.trim();
  const bonusText = bonus === undefined ? message('sheet.play.noValue') : formatModifier(bonus);
  const attackRollName = messageWith('sheet.play.attackRoll', {
    name: attackName,
    value: bonusText,
  });
  const damageRollName = messageWith('sheet.play.damageRoll', {
    name: attackName,
    value: damage.trim(),
  });

  return (
    <li className="lg-attack-card" data-testid={`sheet-attack-${slot.slot}`}>
      <div className="lg-attack-card__heading">
        <h3 className="lg-attack-card__name">
          <InlineEdit
            className="lg-attack-card__name-value"
            label={nameLabel}
            path={slot.namePath}
            schema={nameSchema}
            value={name}
            onCommit={commitName}
          />
        </h3>
        {notesSchema !== undefined && notes.trim() !== '' && (
          <p className="lg-attack-card__notes" data-testid={`sheet-attack-notes-${slot.slot}`}>
            <InlineEdit
              className="lg-attack-card__notes-value"
              label={notesLabel}
              path={slot.notesPath}
              schema={notesSchema}
              value={notes}
              onCommit={commitNotes}
            />
          </p>
        )}
      </div>
      <dl className="lg-attack-card__stats">
        <div className="lg-attack-card__stat">
          <dt className="lg-attack-card__label">{bonusLabel}</dt>
          <dd className="lg-attack-card__value">
            <RollableValue
              rowKey={slot.rowKey}
              sectionKey="attacks"
              label={bonusLabel}
              text={bonusText}
              allowRoll={bonus !== undefined}
              className="lg-attack-card__roll"
              accessibleName={attackRollName}
              icon={<DieIcon shape="d20" />}
            />
            <InlineEdit
              className="lg-attack-card__edit"
              display={<PencilIcon className="lg-attack-card__pencil" />}
              label={bonusLabel}
              path={slot.bonusPath}
              schema={bonusSchema}
              value={bonus === undefined ? '' : String(bonus)}
              inputMode="numeric"
              onCommit={(text) => {
                const next = parseAttackBonus(text);
                if (next !== undefined) {
                  commitBonus(next);
                }
              }}
            />
          </dd>
        </div>
        <div className="lg-attack-card__stat">
          <dt className="lg-attack-card__label">{damageLabel}</dt>
          <dd className="lg-attack-card__value">
            {damageExpression === undefined ? (
              <span className="lg-rollable lg-rollable--still" data-rollable="false">
                {damage.trim() === '' ? message('sheet.play.noValue') : damage}
              </span>
            ) : (
              <button
                type="button"
                className="lg-rollable lg-rollable--button lg-attack-card__roll"
                aria-label={damageRollName}
                title={damageRollName}
                onClick={() => rollDamage(damageExpression)}
              >
                {damage}
              </button>
            )}
            <InlineEdit
              className="lg-attack-card__edit"
              display={<PencilIcon className="lg-attack-card__pencil" />}
              label={damageLabel}
              path={slot.damagePath}
              schema={damageSchema}
              value={damage}
              onCommit={commitDamage}
            />
          </dd>
        </div>
      </dl>
    </li>
  );
}

export function FantasyAttacks({ showAdd = true }: { readonly showAdd?: boolean }) {
  const { store, template, fields } = usePresetContext();
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);
  const [revealed, setRevealed] = useState<readonly number[]>([]);
  const present = useMemo(() => ATTACK_SLOTS.filter((slot) => fields.has(slot.namePath)), [fields]);
  const visibility = attackVisibility(
    present.map((slot) => valueAtPath(state.data, slot.namePath)),
    revealed,
  );
  const attacksNode = template.fields.attacks;

  if (present.length === 0) {
    return null;
  }

  const shown = visibility.shown.flatMap((position) => {
    const slot = present[position - 1];
    return slot === undefined ? [] : [slot];
  });
  const nextPosition = showAdd ? visibility.next : undefined;

  if (!showAdd && shown.length === 0) {
    return null;
  }

  return (
    <section
      className="lg-play-section lg-attacks"
      data-testid="sheet-attacks"
      aria-label={groupDisplayName(
        'attacks',
        attacksNode?.kind === 'group' ? attacksNode.label : undefined,
      )}
    >
      <h2 className="lg-play-section__title">
        {groupDisplayName('attacks', attacksNode?.kind === 'group' ? attacksNode.label : undefined)}
      </h2>
      <ul className="lg-attacks__list">
        {shown.map((slot) => (
          <AttackCard key={slot.slot} slot={slot} />
        ))}
      </ul>
      {nextPosition !== undefined && (
        <Button
          className="lg-attacks__add"
          variant="ghost"
          size="sm"
          label={messageWith('sheet.play.addAttack', { count: visibility.free.length })}
          onClick={() =>
            setRevealed((current) =>
              nextPosition === undefined ? current : [...current, nextPosition],
            )
          }
        />
      )}
    </section>
  );
}
