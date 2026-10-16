export interface RightPanelProps {
  /** When true the panel is hidden — and it is the first region to go (FND-03). */
  readonly collapsed: boolean;
}

/**
 * The contextual panel (order 3), the first region to collapse as the viewport
 * narrows (FND-03). When visible it is part of the accessibility tree.
 */
export function RightPanel({ collapsed }: RightPanelProps) {
  if (collapsed) {
    return null;
  }
  return (
    <aside className="lg-panel" aria-label="Context">
      <h2 className="lg-panel__title">Context</h2>
      <p className="lg-panel__empty">Details for the open workspace will appear here.</p>
    </aside>
  );
}
