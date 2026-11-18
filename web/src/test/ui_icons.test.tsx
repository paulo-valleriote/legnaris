import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/react';
import {
  ArrowRightIcon,
  BackIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CloseIcon,
  DiceIcon,
  MenuIcon,
  MinusIcon,
  OpenIcon,
  PencilIcon,
  PlusIcon,
} from '../ui/icons';

const ICONS = [
  ['ChevronRight', ChevronRightIcon],
  ['ChevronLeft', ChevronLeftIcon],
  ['ChevronDown', ChevronDownIcon],
  ['Back', BackIcon],
  ['Close', CloseIcon],
  ['Plus', PlusIcon],
  ['Minus', MinusIcon],
  ['Pencil', PencilIcon],
  ['ArrowRight', ArrowRightIcon],
  ['Open', OpenIcon],
  ['Check', CheckIcon],
  ['Dice', DiceIcon],
  ['Menu', MenuIcon],
] as const;

describe('icon module', () => {
  it.each(ICONS)('Happy path: %s draws one hidden 24-unit currentColour glyph', (_name, Icon) => {
    const { container } = render(<Icon />);
    const svg = container.querySelector('svg')!;
    expect(svg).not.toBeNull();
    expect(svg).toHaveAttribute('viewBox', '0 0 24 24');
    expect(svg).toHaveAttribute('aria-hidden', 'true');
    expect(svg).toHaveAttribute('stroke', 'currentColor');
    expect(svg).toHaveAttribute('stroke-width', '1.6');
    expect(svg).toHaveAttribute('width', '1em');
    expect(svg).toHaveAttribute('height', '1em');
    expect(svg.querySelector('path, rect, circle')).not.toBeNull();
  });

  it('Happy path: the Open glyph is the arrow the catalog rows use', () => {
    const { container } = render(<OpenIcon />);
    expect(container.querySelector('svg')!.innerHTML).toBe(
      render(<ArrowRightIcon />).container.querySelector('svg')!.innerHTML,
    );
  });

  it('Happy path: the Back glyph is the left chevron', () => {
    const { container } = render(<BackIcon />);
    expect(container.querySelector('svg')!.innerHTML).toBe(
      render(<ChevronLeftIcon />).container.querySelector('svg')!.innerHTML,
    );
  });

  it('Happy path: a size override and a class are both honoured', () => {
    const { container } = render(<CloseIcon size="1.25em" className="lg-test__icon" />);
    const svg = container.querySelector('svg')!;
    expect(svg).toHaveAttribute('width', '1.25em');
    expect(svg).toHaveAttribute('class', 'lg-test__icon');
  });
});
