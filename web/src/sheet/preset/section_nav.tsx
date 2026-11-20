import { useCallback, useEffect, useRef, useState } from 'react';
import { message, messageWith } from '../../l10n/strings';
import {
  PRESET_SELECTOR,
  atScrollEnd,
  currentSectionKey,
  prefersReducedMotion,
  presentSections,
  revealHeading,
  sectionHeading,
  type PresetSectionTarget,
} from './section_nav_targets';
import './section_nav.css';

const REACHED_LINE_OFFSET = 96;

export function PresetSectionNav({ after }: { readonly after: string }) {
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
      setTargets(presentSections(preset, after));
    };
    scan();
  }, [after, root]);

  useEffect(() => {
    const preset = root();
    if (preset === undefined || targets.length === 0) {
      return undefined;
    }
    const update = (): void => {
      const positions = targets.flatMap(({ key }) => {
        const heading = sectionHeading(preset, key);
        return heading === undefined ? [] : [{ key, top: heading.getBoundingClientRect().top }];
      });
      const page = document.documentElement;
      const ended = atScrollEnd({
        scrollTop: window.scrollY || page.scrollTop,
        clientHeight: page.clientHeight,
        scrollHeight: page.scrollHeight,
      });
      setActiveKey(currentSectionKey(positions, REACHED_LINE_OFFSET, ended));
    };
    update();
    window.addEventListener('scroll', update, { passive: true });
    window.addEventListener('resize', update);
    return () => {
      window.removeEventListener('scroll', update);
      window.removeEventListener('resize', update);
    };
  }, [root, targets]);

  const jump = (key: string): void => {
    const preset = root();
    if (preset === undefined) {
      return;
    }
    const heading = sectionHeading(preset, key);
    if (heading === undefined) {
      return;
    }
    setActiveKey(key);
    revealHeading(heading, prefersReducedMotion());
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
                  onClick={() => jump(target.key)}
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
