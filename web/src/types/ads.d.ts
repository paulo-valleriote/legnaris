interface Window {
  __tcfapi?: (
    command: string,
    version: number,
    callback: (tcData: unknown, success: boolean) => void,
    parameter?: number,
  ) => void;
  adsbygoogle?: unknown[] & { requestNonPersonalizedAds?: number };
  googlefcPresent?: boolean;
}
