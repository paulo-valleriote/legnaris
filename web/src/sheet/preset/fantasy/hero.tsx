import { message, messageWith } from '../../../l10n/strings';
import { MinusIcon, PlusIcon } from '../../../ui/icons';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetContext,
  usePresetLabel,
  usePresetSchema,
  usePresetValue,
} from '../preset_context';
import { numberValue, textValue } from '../values';
import { parseProficiencyBonus } from './identity';
import { XpBar } from './xp_panel';

const LEVEL_MIN = 1;
const LEVEL_MAX = 20;

function clampLevel(level: number): number {
  return Math.min(LEVEL_MAX, Math.max(LEVEL_MIN, level));
}

export function FantasyHero() {
  const { sheet } = usePresetContext();
  const characterName = textValue(usePresetValue('identity.characterName'));
  const playerName = textValue(usePresetValue('identity.playerName'));
  const ancestry = textValue(usePresetValue('identity.ancestry'));
  const origin = textValue(usePresetValue('identity.origin'));
  const level = numberValue(usePresetValue('identity.level'));
  const proficiency = numberValue(usePresetValue('identity.proficiencyBonus'));
  const commitCharacterName = usePresetCommit('identity.characterName');
  const commitPlayerName = usePresetCommit('identity.playerName');
  const commitAncestry = usePresetCommit('identity.ancestry');
  const commitOrigin = usePresetCommit('identity.origin');
  const commitLevel = usePresetCommit('identity.level');
  const commitProficiency = usePresetCommit('identity.proficiencyBonus');
  const characterNameLabel = usePresetLabel('identity.characterName');
  const playerNameLabel = usePresetLabel('identity.playerName');
  const ancestryLabel = usePresetLabel('identity.ancestry');
  const originLabel = usePresetLabel('identity.origin');
  const levelLabel = usePresetLabel('identity.level');
  const proficiencyLabel = usePresetLabel('identity.proficiencyBonus');
  const proficiencySchema = usePresetSchema('identity.proficiencyBonus');
  const shownLevel = level ?? LEVEL_MIN;

  return (
    <div className="lg-preset-hero" data-testid="sheet-play-hero">
      <h1 className="lg-sheet__title" data-testid="sheet-name">
        <InlineEdit
          className="lg-preset-hero__title"
          label={characterNameLabel}
          path="identity.characterName"
          value={characterName}
          fallback={sheet.name}
          onCommit={commitCharacterName}
        />
      </h1>
      <p className="lg-preset-hero__meta">
        <InlineEdit
          className="lg-preset-hero__meta-item"
          label={ancestryLabel}
          path="identity.ancestry"
          value={ancestry}
          onCommit={commitAncestry}
        />
        <span className="lg-preset-hero__separator" aria-hidden="true">
          ·
        </span>
        <InlineEdit
          className="lg-preset-hero__meta-item"
          label={originLabel}
          path="identity.origin"
          value={origin}
          onCommit={commitOrigin}
        />
        <span className="lg-preset-hero__separator" aria-hidden="true">
          ·
        </span>
        <span className="lg-preset-hero__level">
          <button
            type="button"
            className="lg-preset-hero__step"
            aria-label={messageWith('engine.decrease', { label: levelLabel })}
            data-edit-path="identity.level"
            disabled={shownLevel <= LEVEL_MIN}
            onClick={() => commitLevel(clampLevel(shownLevel - 1))}
          >
            <MinusIcon />
          </button>
          <span className="lg-preset-hero__level-text" data-testid="sheet-play-level">
            {messageWith('sheet.play.level', { count: shownLevel })}
          </span>
          <button
            type="button"
            className="lg-preset-hero__step"
            aria-label={messageWith('engine.increase', { label: levelLabel })}
            data-edit-path="identity.level"
            disabled={shownLevel >= LEVEL_MAX}
            onClick={() => commitLevel(clampLevel(shownLevel + 1))}
          >
            <PlusIcon />
          </button>
        </span>
      </p>
      <div className="lg-preset-hero__facts">
        {playerName.trim() === '' ? (
          <InlineEdit
            className="lg-preset-hero__player"
            label={playerNameLabel}
            path="identity.playerName"
            value={playerName}
            onCommit={commitPlayerName}
          />
        ) : (
          <p className="lg-preset-hero__played">
            {message('sheet.play.playedBy')}{' '}
            <InlineEdit
              className="lg-preset-hero__player"
              label={playerNameLabel}
              path="identity.playerName"
              value={playerName}
              onCommit={commitPlayerName}
            />
          </p>
        )}
        <span className="lg-preset-hero__badge" data-testid="sheet-play-proficiency">
          <InlineEdit
            className="lg-preset-hero__proficiency"
            label={proficiencyLabel}
            path="identity.proficiencyBonus"
            schema={proficiencySchema}
            inputMode="numeric"
            value={proficiency === undefined ? '' : String(proficiency)}
            display={messageWith('sheet.play.proficiency', {
              modifier: `+${proficiency ?? 0}`,
            })}
            onCommit={(text) => {
              const next = parseProficiencyBonus(text);
              if (next !== undefined) {
                commitProficiency(next);
              }
            }}
          />
        </span>
      </div>
      <XpBar />
    </div>
  );
}
