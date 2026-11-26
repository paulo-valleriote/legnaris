import { Fragment, type ComponentType } from 'react';
import { useNarrowViewport } from '../inline_edit';
import { PendingChangesCard } from './pending_card';
import { usePresetContext } from './preset_context';
import { PresetSectionNav } from './section_nav';
import type { PresetLayoutBlock, PresetSectionNavTarget, SheetPreset } from './types';
import './preset.css';

function sectionComponents(preset: SheetPreset): ReadonlyMap<string, ComponentType> {
  return new Map(preset.sections.map(({ key, Component }) => [key, Component]));
}

function SectionList({
  keys,
  components,
  wrap,
  navAfter,
  navTargets,
}: {
  readonly keys: readonly string[];
  readonly components: ReadonlyMap<string, ComponentType>;
  readonly wrap: boolean;
  readonly navAfter?: string;
  readonly navTargets?: readonly PresetSectionNavTarget[];
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
            {navAfter === key && <PresetSectionNav skipKey={key} targets={navTargets} />}
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
  navTargets,
  columnsClassName,
}: {
  readonly block: PresetLayoutBlock;
  readonly components: ReadonlyMap<string, ComponentType>;
  readonly wrap: boolean;
  readonly navAfter?: string;
  readonly navTargets?: readonly PresetSectionNavTarget[];
  readonly columnsClassName?: string;
}) {
  if ('columns' in block) {
    const className = ['lg-preset__columns', columnsClassName]
      .filter((entry): entry is string => entry !== undefined)
      .join(' ');
    return (
      <div className={className}>
        {block.columns.map((column, position) => (
          <div className="lg-preset__column" key={`column-${position}`}>
            <SectionList
              keys={column}
              components={components}
              wrap={wrap}
              navAfter={navAfter}
              navTargets={navTargets}
            />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="lg-preset__block">
      <SectionList
        keys={block.sections}
        components={components}
        wrap={wrap}
        navAfter={navAfter}
        navTargets={navTargets}
      />
    </div>
  );
}

export function PresetPlayView({ preset }: { readonly preset: SheetPreset }) {
  const { editing } = usePresetContext();
  const narrow = useNarrowViewport();
  const components = sectionComponents(preset);
  const layout = narrow && preset.narrowLayout !== undefined ? preset.narrowLayout : preset.layout;
  const nav = preset.sectionNav;
  const navTargets = nav?.targets;
  const wrap = narrow || nav !== undefined;
  const navAfter = narrow ? nav?.after : undefined;
  const topNav = !narrow && nav !== undefined;

  return (
    <div className={`lg-preset${editing ? '' : ' lg-preset--locked'}`} data-testid="sheet-play">
      {layout === undefined ? (
        <>
          {topNav && <PresetSectionNav targets={navTargets} />}
          <SectionList
            keys={preset.sections.map(({ key }) => key)}
            components={components}
            wrap={wrap}
            navAfter={navAfter}
            navTargets={navTargets}
          />
        </>
      ) : (
        <div className="lg-preset__body">
          {topNav && <PresetSectionNav targets={navTargets} />}
          {layout.map((block, index) => (
            <LayoutBlock
              key={index}
              block={block}
              components={components}
              wrap={wrap}
              navAfter={navAfter}
              navTargets={navTargets}
              columnsClassName={preset.columnsClassName}
            />
          ))}
        </div>
      )}
      <PendingChangesCard />
    </div>
  );
}
