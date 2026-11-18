export interface IconProps {
  className?: string;
  size?: string;
}

const BASE = {
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.6,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
  'aria-hidden': true,
  focusable: false,
};

function dimensions(size: string | undefined): { width: string; height: string } {
  const value = size ?? '1em';
  return { width: value, height: value };
}

export function ChevronRightIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="m9 6 6 6-6 6" />
    </svg>
  );
}

export function ChevronLeftIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="m15 6-6 6 6 6" />
    </svg>
  );
}

export const BackIcon = ChevronLeftIcon;

export function ChevronDownIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="m6 9 6 6 6-6" />
    </svg>
  );
}

export function MinusIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="M5 12h14" />
    </svg>
  );
}

export function PencilIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="M15.5 4.5 19.5 8.5 9 19H5v-4z" />
      <path d="M13 7l4 4" />
    </svg>
  );
}

export function CloseIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="M6 6l12 12" />
      <path d="M18 6 6 18" />
    </svg>
  );
}

export function PlusIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="M12 5v14" />
      <path d="M5 12h14" />
    </svg>
  );
}

export function ArrowRightIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="M5 12h14" />
      <path d="m13 6 6 6-6 6" />
    </svg>
  );
}

export const OpenIcon = ArrowRightIcon;

export function CheckIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="m5 12 4 4L19 6" />
    </svg>
  );
}

export function DiceIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <rect x="4" y="4" width="16" height="16" rx="3" />
      <circle cx="9" cy="9" r="1" fill="currentColor" stroke="none" />
      <circle cx="15" cy="15" r="1" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1" fill="currentColor" stroke="none" />
    </svg>
  );
}

export function MenuIcon({ className, size }: IconProps) {
  return (
    <svg className={className} {...BASE} {...dimensions(size)}>
      <path d="M5 7h14" />
      <path d="M5 12h14" />
      <path d="M5 17h14" />
    </svg>
  );
}
