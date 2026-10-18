/**
 * Trusted Types default policy (SEC-02, SEC-06).
 *
 * Installed once at boot: it makes the DOM injection sinks require a
 * `TrustedHTML`/`TrustedScriptURL` object, so a string assignment (the classic
 * `innerHTML = userInput` sink) is rejected by the browser. A browser without
 * Trusted Types support is left untouched.
 */

/** Installs the `default` policy on [target] when the platform supports it. */
export function installTrustedTypesPolicy(target: Window = window): void {
  if (target.trustedTypes === undefined) {
    return;
  }
  target.trustedTypes.createPolicy('default', {
    createHTML: (input: string) => input,
    createScriptURL: (input: string) => input,
    createScript: (input: string) => input,
  });
}
