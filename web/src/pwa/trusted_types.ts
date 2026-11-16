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
