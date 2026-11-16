export function SectionChevron({ className }: { readonly className: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <polyline points="6,9 12,15 18,9" fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
