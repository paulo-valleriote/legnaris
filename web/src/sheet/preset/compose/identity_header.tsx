import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetContext,
  usePresetLabel,
  usePresetValue,
} from '../preset_context';
import { textValue } from '../values';
import type { PresetHeaderLevel, PresetIdentityHeader } from './types';
import './compose.css';

export interface IdentityHeaderProps {
  readonly header: PresetIdentityHeader;
  readonly level?: PresetHeaderLevel;
}

export function IdentityHeader({ header, level = 1 }: IdentityHeaderProps) {
  const { sheet } = usePresetContext();
  const name = textValue(usePresetValue(header.name));
  const subtitle = textValue(usePresetValue(header.subtitle ?? header.name));
  const nameLabel = usePresetLabel(header.name);
  const subtitleLabel = usePresetLabel(header.subtitle ?? header.name);
  const commitName = usePresetCommit(header.name);
  const commitSubtitle = usePresetCommit(header.subtitle ?? header.name);
  const Heading = level === 2 ? 'h2' : 'h1';

  return (
    <div className="lg-compose-header">
      <Heading className="lg-compose-header__name" data-testid="sheet-name">
        <InlineEdit
          className="lg-compose-header__title"
          label={nameLabel}
          path={header.name}
          value={name}
          fallback={sheet.name}
          onCommit={commitName}
        />
      </Heading>
      {header.subtitle === undefined ? null : (
        <p className="lg-compose-header__subtitle">
          <InlineEdit
            className="lg-compose-header__subtitle-item"
            label={subtitleLabel}
            path={header.subtitle}
            value={subtitle}
            onCommit={commitSubtitle}
          />
        </p>
      )}
    </div>
  );
}
