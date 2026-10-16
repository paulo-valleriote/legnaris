/**
 * The content region's designed empty state (FND-01). Until the catalog
 * feature lands, the root route shows this instead of placeholder rows or demo
 * data.
 */
export function EmptyWorkspace() {
  return (
    <section
      className="lg-empty"
      aria-labelledby="empty-workspace-title"
      data-testid="empty-workspace"
    >
      <h1 className="lg-empty__title" id="empty-workspace-title">
        No workspace open
      </h1>
      <p className="lg-empty__text">
        Templates and sheets will appear here once the catalog is available. Nothing is shown until
        then.
      </p>
    </section>
  );
}
