/**
 * The star glyph shared by the pin toggle (Arrange mode) and the pinned mark beside a Row's
 * label. Drawn on `currentColor`; the caller decides outline or fill through CSS.
 */
export function PinStar({ className }: { readonly className: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <polygon
        points="12,2.8 14.9,8.9 21.5,9.7 16.6,14.3 17.9,20.9 12,17.6 6.1,20.9 7.4,14.3 2.5,9.7 9.1,8.9"
        strokeLinejoin="round"
      />
    </svg>
  );
}
