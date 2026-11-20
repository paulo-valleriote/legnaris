import { Fragment, type ComponentType } from 'react';
import { useNarrowViewport } from '../inline_edit';
import { PendingChangesCard } from './pending_card';
import { usePresetContext } from './preset_context';
import { PresetSectionNav } from './section_nav';
import type { PresetLayoutBlock, SheetPreset } from './types';
import './preset.css';

function sectionComponents(preset: SheetPreset): ReadonlyMap<string, ComponentType> {
  return new Map(preset.sections.map(({ key, Component }) => [key, Component]));
}

function SectionList({
  keys,
  components,
  wrap,
  navAfter,
}: {
  readonly keys: readonly string[];
  readonly components: ReadonlyMap<string, ComponentType>;
  readonly wrap: boolean;
  readonly navAfter?: string;
}) {
  return (
    <>
      {keys.map((key) => {
        const Component = components.get(key);
        return (
          <Fragment key={key}>
            {Component === undefined ? null : wrap ? (
              <div className="lg-preset__section" data-preset-section={key}>
                <Component />
              </div>
            ) : (
              <Component />
            )}
            {navAfter === key && <PresetSectionNav after={key} />}
          </Fragment>
        );
      })}
    </>
  );
}

function LayoutBlock({
  block,
  components,
  wrap,
  navAfter,
}: {
  readonly block: PresetLayoutBlock;
  readonly components: ReadonlyMap<string, ComponentType>;
  readonly wrap: boolean;
  readonly navAfter?: string;
}) {
  if ('columns' in block) {
    return (
      <div className="lg-preset__columns">
        {block.columns.map((column, position) => (
          <div className="lg-preset__column" key={`column-${position}`}>
            <SectionList keys={column} components={components} wrap={wrap} navAfter={navAfter} />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="lg-preset__block">
      <SectionList keys={block.sections} components={components} wrap={wrap} navAfter={navAfter} />
    </div>
  );
}

export function PresetPlayView({ preset }: { readonly preset: SheetPreset }) {
  const { sheet } = usePresetContext();
  const narrow = useNarrowViewport();
  const components = sectionComponents(preset);
  const statBlock = preset.statBlock?.when(sheet.kind) ? preset.statBlock : undefined;
  const layout = narrow && preset.narrowLayout !== undefined ? preset.narrowLayout : preset.layout;
  const navAfter = narrow ? preset.sectionNav?.after : undefined;

  return (
    <div className="lg-preset" data-testid="sheet-play">
      {statBlock !== undefined ? (
        <statBlock.Body />
      ) : layout === undefined ? (
        preset.sections.map(({ key, Component }) => <Component key={key} />)
      ) : (
        <div className="lg-preset__body">
          {layout.map((block, index) => (
            <LayoutBlock
              key={index}
              block={block}
              components={components}
              wrap={narrow}
              navAfter={navAfter}
            />
          ))}
        </div>
      )}
      {statBlock === undefined && preset.Vitals !== undefined && <preset.Vitals />}
      <PendingChangesCard />
    </div>
  );
}
