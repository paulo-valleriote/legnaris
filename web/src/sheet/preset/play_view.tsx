import type { ComponentType } from 'react';
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
  components,
}: {
  readonly block: PresetLayoutBlock;
  readonly components: ReadonlyMap<string, ComponentType>;
}) {
  if ('columns' in block) {
    return (
      <div className="lg-preset__columns">
        {block.columns.map((column, position) => (
          <div className="lg-preset__column" key={`column-${position}`}>
            <SectionList keys={column} components={components} />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="lg-preset__block">
      <SectionList keys={block.sections} components={components} />
    </div>
  );
}

export function PresetPlayView({ preset }: { readonly preset: SheetPreset }) {
  const { sheet } = usePresetContext();
  const components = sectionComponents(preset);
  const statBlock = preset.statBlock?.when(sheet.kind) ? preset.statBlock : undefined;

  return (
    <div className="lg-preset" data-testid="sheet-play">
      {statBlock !== undefined ? (
        <statBlock.Body />
      ) : preset.layout === undefined ? (
        preset.sections.map(({ key, Component }) => <Component key={key} />)
      ) : (
        <div className="lg-preset__body">
          {preset.layout.map((block, index) => (
            <LayoutBlock key={index} block={block} components={components} />
          ))}
        </div>
      )}
      <PendingChangesCard />
    </div>
  );
}
