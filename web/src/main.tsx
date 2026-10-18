import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AppRoutes } from './app/router';
import { AppProviders } from './app/providers';
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

createRoot(container).render(
  <StrictMode>
    <BrowserRouter>
      <AppProviders>
        <AppRoutes />
        <UpdatePrompt />
      </AppProviders>
    </BrowserRouter>
  </StrictMode>,
);

registerServiceWorker();
