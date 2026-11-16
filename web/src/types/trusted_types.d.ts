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
