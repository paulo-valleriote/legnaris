import type { ComponentType } from 'react';
import { PendingChangesCard } from './pending_card';
import type { PresetLayoutBlock, SheetPreset } from './types';
import './preset.css';

function sectionComponents(preset: SheetPreset): ReadonlyMap<string, ComponentType> {
  return new Map(preset.sections.map(({ key, Component }) => [key, Component]));
}

function SectionList({
  keys,
  components,
}: {
  readonly keys: readonly string[];
  readonly components: ReadonlyMap<string, ComponentType>;
}) {
  return (
    <>
      {keys.map((key) => {
        const Component = components.get(key);
        return Component === undefined ? null : <Component key={key} />;
      })}
    </>
  );
}

function LayoutBlock({
  block,
  index,
  components,
}: {
  readonly block: PresetLayoutBlock;
  readonly index: number;
  readonly components: ReadonlyMap<string, ComponentType>;
}) {
  if ('columns' in block) {
    return (
      <div className="lg-preset__columns" key={`columns-${index}`}>
        {block.columns.map((column, position) => (
          <div className="lg-preset__column" key={`column-${position}`}>
            <SectionList keys={column} components={components} />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="lg-preset__block" key={`block-${index}`}>
      <SectionList keys={block.sections} components={components} />
    </div>
  );
}

export function PresetPlayView({ preset }: { readonly preset: SheetPreset }) {
  const components = sectionComponents(preset);

  return (
    <div className="lg-preset" data-testid="sheet-play">
      {preset.layout === undefined ? (
        preset.sections.map(({ key, Component }) => <Component key={key} />)
      ) : (
        <div className="lg-preset__body">
          {preset.layout.map((block, index) => (
            <LayoutBlock key={index} block={block} index={index} components={components} />
          ))}
        </div>
      )}
      <PendingChangesCard />
    </div>
  );
}
