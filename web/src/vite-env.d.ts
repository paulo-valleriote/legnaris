/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/client" />
/// <reference types="vite-plugin-pwa/react" />

interface ImportMetaEnv {
  /** Build-time override for the API base address; empty means the HTTPS default. */
  readonly VITE_API_BASE_URL?: string;
}
