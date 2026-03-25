import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '@evoapi/design-system/dist/index.css';
import './styles/globals.css';
import App from './App';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
