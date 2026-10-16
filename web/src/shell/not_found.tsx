import { Link } from 'react-router-dom';

/**
 * The designed not-found state for undeclared paths (FND-08). It is rendered
 * inside the shell, so an unknown route never produces a blank page or the
 * host's 404.
 */
export function NotFound() {
  return (
    <section className="lg-empty" aria-labelledby="not-found-title" data-testid="not-found">
      <h1 className="lg-empty__title" id="not-found-title">
        Page not found
      </h1>
      <p className="lg-empty__text">
        The address you opened does not match a screen.{' '}
        <Link className="lg-notfound__link" to="/">
          Return to the workspace
        </Link>
        .
      </p>
    </section>
  );
}
