/**
 * Trusted Types globals (SEC-06).
 *
 * The TypeScript DOM library shipped with this toolchain does not declare
 * `Window.trustedTypes`, so the client declares the slice it touches: the
 * `createPolicy` factory that installs the `default` policy at boot.
 */

interface Window {
  readonly trustedTypes?: {
    createPolicy(
      name: string,
      rules?: {
        createHTML?: (input: string) => string;
        createScriptURL?: (input: string) => string;
        createScript?: (input: string) => string;
      },
    ): unknown;
  };
}
