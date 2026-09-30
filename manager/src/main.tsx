import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '@/stores/ui'; // applies the saved theme before the first render
import './styles.css';
import { App } from './app';
import { ErrorBoundary } from '@/components/error-boundary';

createRoot(document.getElementById('root') as HTMLElement).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
);
