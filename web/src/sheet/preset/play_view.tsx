import type { ComponentType } from 'react';
import { useNarrowViewport } from '../inline_edit';
import { PendingChangesCard } from './pending_card';
import { usePresetContext } from './preset_context';
import type { PresetLayoutBlock, SheetPreset } from './types';
import './preset.css';

function sectionComponents(preset: SheetPreset): ReadonlyMap<string, ComponentType> {
  return new Map(preset.sections.map(({ key, Component }) => [key, Component]));
}

function SectionList({
  keys,
  components,
  wrap,
}: {
  readonly keys: readonly string[];
  readonly components: ReadonlyMap<string, ComponentType>;
  readonly wrap: boolean;
}) {
  return (
    <>
      {keys.map((key) => {
        const Component = components.get(key);
        if (Component === undefined) {
          return null;
        }
        return wrap ? (
          <div className="lg-preset__section" data-preset-section={key} key={key}>
            <Component />
          </div>
        ) : (
          <Component key={key} />
        );
      })}
    </>
  );
}

function LayoutBlock({
  block,
  components,
  wrap,
}: {
  readonly block: PresetLayoutBlock;
  readonly components: ReadonlyMap<string, ComponentType>;
  readonly wrap: boolean;
}) {
  if ('columns' in block) {
    return (
      <div className="lg-preset__columns">
        {block.columns.map((column, position) => (
          <div className="lg-preset__column" key={`column-${position}`}>
            <SectionList keys={column} components={components} wrap={wrap} />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="lg-preset__block">
      <SectionList keys={block.sections} components={components} wrap={wrap} />
    </div>
  );
}

export function PresetPlayView({ preset }: { readonly preset: SheetPreset }) {
  const { sheet } = usePresetContext();
  const narrow = useNarrowViewport();
  const components = sectionComponents(preset);
  const statBlock = preset.statBlock?.when(sheet.kind) ? preset.statBlock : undefined;
  const layout = narrow && preset.narrowLayout !== undefined ? preset.narrowLayout : preset.layout;

  return (
    <div className="lg-preset" data-testid="sheet-play">
      {statBlock !== undefined ? (
        <statBlock.Body />
      ) : layout === undefined ? (
        preset.sections.map(({ key, Component }) => <Component key={key} />)
      ) : (
        <div className="lg-preset__body">
          {layout.map((block, index) => (
            <LayoutBlock key={index} block={block} components={components} wrap={narrow} />
          ))}
        </div>
      )}
      <PendingChangesCard />
    </div>
  );
}
