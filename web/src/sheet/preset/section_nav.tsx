import { useCallback, useEffect, useRef, useState } from 'react';
import { message, messageWith } from '../../l10n/strings';
import {
  PRESET_SELECTOR,
  atScrollEnd,
  currentSectionKey,
  prefersReducedMotion,
  presentSections,
  presentTargets,
  revealTarget,
  sectionAnchor,
  type PresetSectionGroup,
  type PresetSectionTarget,
} from './section_nav_targets';
import type { PresetSectionNavTarget } from './types';
import './section_nav.css';

const REACHED_LINE_OFFSET = 96;
const REACHED_LINE_SLACK = 8;

interface PresetSectionNavProps {
  readonly skipKey?: string;
  readonly targets?: readonly PresetSectionNavTarget[];
}

function resolvedGroups(targets: readonly PresetSectionNavTarget[]): readonly PresetSectionGroup[] {
  return targets.map((target, index) => ({
    id: `${index}-${target.label}`,
    label: message(target.label),
    sections: target.sections,
  }));
}

export function PresetSectionNav({ skipKey, targets: configured }: PresetSectionNavProps) {
  const surfaceRef = useRef<HTMLDivElement>(null);
  const [targets, setTargets] = useState<readonly PresetSectionTarget[]>([]);
  const [activeKey, setActiveKey] = useState<string | undefined>(undefined);

  const root = useCallback(
    (): ParentNode | undefined => surfaceRef.current?.closest(PRESET_SELECTOR) ?? undefined,
    [],
  );

  useEffect(() => {
    const preset = root();
    if (preset === undefined) {
      return;
    }
    const scan = (): void => {
      setTargets(
        configured === undefined
          ? presentSections(preset, skipKey ?? '')
          : presentTargets(preset, resolvedGroups(configured), skipKey),
      );
    };
    scan();
  }, [configured, root, skipKey]);

  useEffect(() => {
    const preset = root();
    if (preset === undefined || targets.length === 0) {
      return undefined;
    }
    const update = (): void => {
      const anchors = targets.flatMap(({ key, sectionKey }) => {
        const anchor = sectionAnchor(preset, sectionKey);
        return anchor === undefined ? [] : [{ key, anchor }];
      });
      anchors.sort((left, right) => {
        const relation = left.anchor.compareDocumentPosition(right.anchor);
        if ((relation & Node.DOCUMENT_POSITION_FOLLOWING) !== 0) {
          return -1;
        }
        if ((relation & Node.DOCUMENT_POSITION_PRECEDING) !== 0) {
          return 1;
        }
        return 0;
      });
      const positions = anchors.map(({ key, anchor }) => ({
        key,
        top: anchor.getBoundingClientRect().top,
      }));
      const page = document.documentElement;
      const ended = atScrollEnd({
        scrollTop: window.scrollY || page.scrollTop,
        clientHeight: page.clientHeight,
        scrollHeight: page.scrollHeight,
      });
      const margin =
        anchors.length === 0
          ? 0
          : parseFloat(getComputedStyle(anchors[0].anchor).scrollMarginBlockStart);
      const line = Math.max(
        REACHED_LINE_OFFSET,
        (Number.isFinite(margin) ? margin : 0) + REACHED_LINE_SLACK,
      );
      setActiveKey(currentSectionKey(positions, line, ended));
    };
    update();
    window.addEventListener('scroll', update, { passive: true });
    window.addEventListener('resize', update);
    return () => {
      window.removeEventListener('scroll', update);
      window.removeEventListener('resize', update);
    };
  }, [root, targets]);

  const jump = (target: PresetSectionTarget): void => {
    const preset = root();
    if (preset === undefined) {
      return;
    }
    const anchor = sectionAnchor(preset, target.sectionKey);
    if (anchor === undefined) {
      return;
    }
    setActiveKey(target.key);
    revealTarget(anchor, prefersReducedMotion());
  };

  return (
    <div
      className="lg-preset-nav"
      data-testid="sheet-section-nav"
      hidden={targets.length === 0}
      ref={surfaceRef}
    >
      {targets.length > 0 && (
        <nav className="lg-preset-nav__nav" aria-label={message('sheet.sections')}>
          <ul className="lg-preset-nav__list">
            {targets.map((target) => (
              <li key={target.key}>
                <button
                  type="button"
                  className="lg-preset-nav__chip"
                  aria-current={target.key === activeKey ? 'location' : undefined}
                  aria-label={messageWith('sheet.sectionLink', { label: target.label })}
                  onClick={() => jump(target)}
                >
                  {target.label}
                </button>
              </li>
            ))}
          </ul>
        </nav>
      )}
    </div>
  );
}
