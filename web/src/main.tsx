import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AppRoutes } from './app/router';
import { UpdatePrompt } from './pwa/update_prompt';
import { registerServiceWorker } from './pwa/register_sw';
import './theme/tokens.css';
import './theme/base.css';

const container = document.getElementById('root');
if (!container) {
  throw new Error('Root container #root is missing from index.html');
}

createRoot(container).render(
  <StrictMode>
    <BrowserRouter>
      <AppRoutes />
      <UpdatePrompt />
    </BrowserRouter>
  </StrictMode>,
);

registerServiceWorker();
