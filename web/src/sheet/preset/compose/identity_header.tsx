import type { ReactNode } from 'react';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetContext,
  usePresetLabel,
  usePresetSchema,
  usePresetValue,
} from '../preset_context';
import { textValue } from '../values';
import type { PresetHeaderLevel, PresetIdentityHeader } from './types';
import './compose.css';

export interface IdentityHeaderProps {
  readonly header: PresetIdentityHeader;
  readonly level?: PresetHeaderLevel;
  readonly trailing?: ReactNode;
}

export function IdentityHeader({ header, level = 1, trailing }: IdentityHeaderProps) {
  const { sheet, editing } = usePresetContext();
  const name = textValue(usePresetValue(header.name));
  const subtitle = textValue(usePresetValue(header.subtitle ?? header.name));
  const nameLabel = usePresetLabel(header.name);
  const subtitleLabel = usePresetLabel(header.subtitle ?? header.name);
  const commitName = usePresetCommit(header.name);
  const commitSubtitle = usePresetCommit(header.subtitle ?? header.name);
  const nameSchema = usePresetSchema(header.name);
  const subtitleSchema = usePresetSchema(header.subtitle ?? header.name);
  const Heading = level === 2 ? 'h2' : 'h1';

  return (
    <div className="lg-compose-header">
      <div className="lg-compose-header__identity">
        <Heading className="lg-compose-header__name" data-testid="sheet-name">
          <InlineEdit
            className="lg-compose-header__title"
            label={nameLabel}
            path={header.name}
            schema={nameSchema}
            value={name}
            fallback={sheet.name}
            locked={!editing}
            onCommit={commitName}
          />
        </Heading>
        {header.subtitle === undefined ? null : (
          <p className="lg-compose-header__subtitle">
            <InlineEdit
              className="lg-compose-header__subtitle-item"
              label={subtitleLabel}
              path={header.subtitle}
              schema={subtitleSchema}
              value={subtitle}
              locked={!editing}
              onCommit={commitSubtitle}
            />
          </p>
        )}
      </div>
      {trailing === undefined ? null : (
        <div className="lg-compose-header__trailing">{trailing}</div>
      )}
    </div>
  );
}
