import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AppRoutes } from './app/router';
import { AppProviders } from './app/providers';
import { loadApiConfiguration } from './api/runtime_config';
import { UpdatePrompt } from './pwa/update_prompt';
import { registerServiceWorker } from './pwa/register_sw';
import { installTrustedTypesPolicy } from './pwa/trusted_types';
import './theme/tokens.css';
import './theme/base.css';

// Trusted Types default policy (F9-T005 / SEC-02, SEC-06): closes DOM-based XSS
// sinks. No inline HTML assignment without a TrustedHTML object after this point.
installTrustedTypesPolicy();

const container = document.getElementById('root');
if (!container) {
  throw new Error('Root container #root is missing from index.html');
}
const root = container;

function showConfigurationError(error: unknown): void {
  const message = error instanceof Error ? error.message : 'unknown error';
  const alert = document.createElement('p');
  alert.setAttribute('role', 'alert');
  alert.textContent = `Legnaris could not start: ${message}. Reload the page or contact the administrator.`;
  root.replaceChildren(alert);
}

async function start(): Promise<void> {
  const config = await loadApiConfiguration({
    fetcher: (input, init) => fetch(input, init),
    buildValue: import.meta.env.VITE_API_BASE_URL,
  });

  createRoot(root).render(
    <StrictMode>
      <BrowserRouter>
        <AppProviders apiBaseUrl={config.baseUrl}>
          <AppRoutes />
          <UpdatePrompt />
        </AppProviders>
      </BrowserRouter>
    </StrictMode>,
  );

  registerServiceWorker();
}

start().catch(showConfigurationError);
