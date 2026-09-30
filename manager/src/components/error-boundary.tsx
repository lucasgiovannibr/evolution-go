import { Component, type ErrorInfo, type ReactNode } from 'react';
import { RotateCw, TriangleAlert } from 'lucide-react';
import { Button } from '@/components/ui/button';

interface State {
  error: Error | null;
}

/** Last line of defense: a render error shows a recoverable screen instead of a blank page. */
export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Manager crashed:', error, info.componentStack);
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="flex min-h-dvh items-center justify-center p-6">
        <div className="w-full max-w-sm text-center">
          <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-xl bg-danger-soft text-danger">
            <TriangleAlert className="size-6" />
          </span>
          <h1 className="text-lg font-semibold">Algo deu errado</h1>
          <p className="mt-1 text-[13px] text-muted">O painel encontrou um erro inesperado. Recarregar a página normalmente resolve.</p>
          <pre className="mt-4 max-h-28 overflow-auto rounded-control bg-surface-2 p-2 text-left font-mono text-[11px] text-muted">{this.state.error.message}</pre>
          <div className="mt-5 flex justify-center gap-2">
            <Button variant="primary" onClick={() => window.location.reload()}>
              <RotateCw className="size-3.5" />
              Recarregar
            </Button>
            <Button onClick={() => window.location.assign('/manager')}>Ir ao início</Button>
          </div>
        </div>
      </div>
    );
  }
}
