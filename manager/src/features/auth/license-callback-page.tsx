import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { CheckCircle2, Loader2, XCircle } from 'lucide-react';
import { activateLicense, verifyApiKey } from '@/api/session';
import { useAuth } from '@/stores/auth';
import { Button } from '@/components/ui/button';
import { AuthLayout } from './auth-layout';

type Phase = 'activating' | 'success' | 'error';

/** Landing page for the license provider redirect: exchanges ?code for an activation, then signs in. */
export function LicenseCallbackPage() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const { apiUrl, apiKey, setSession } = useAuth.getState();
  const code = params.get('code');
  const [phase, setPhase] = useState<Phase>('activating');
  const [message, setMessage] = useState('');
  const started = useRef(false);

  const run = useCallback(async () => {
    if (!code) {
      setPhase('error');
      setMessage('Código de autorização não encontrado na URL.');
      return;
    }
    if (!apiKey) {
      setPhase('error');
      setMessage('Sessão expirada. Volte ao login e informe a API Key novamente.');
      return;
    }
    setPhase('activating');
    setMessage('');
    try {
      const res = await activateLicense(apiUrl, apiKey, code);
      if (res.status !== 'active') {
        setPhase('error');
        setMessage(res.message || 'Falha ao ativar a licença.');
        return;
      }
      setPhase('success');
      try {
        await verifyApiKey(apiUrl, apiKey);
        setSession({ isAuthenticated: true, licenseState: 'licensed' });
        window.setTimeout(() => navigate('/manager', { replace: true }), 1500);
      } catch {
        setSession({ licenseState: 'licensed' });
        window.setTimeout(() => navigate('/manager/login', { replace: true }), 1500);
      }
    } catch (err) {
      setPhase('error');
      setMessage(err instanceof Error ? err.message : 'Erro ao ativar a licença.');
    }
  }, [code, apiUrl, apiKey, setSession, navigate]);

  useEffect(() => {
    if (started.current) return; // StrictMode runs effects twice; the code is single-use
    started.current = true;
    void run();
  }, [run]);

  return (
    <AuthLayout>
      <div role="status" aria-live="polite" className="flex flex-col items-center gap-4 py-6 text-center">
        {phase === 'activating' ? (
          <>
            <Loader2 className="size-10 animate-spin text-brand" />
            <div>
              <h1 className="text-lg font-semibold">Ativando licença…</h1>
              <p className="mt-1 text-[13px] text-muted">Aguarde enquanto concluímos a ativação.</p>
            </div>
          </>
        ) : null}
        {phase === 'success' ? (
          <>
            <CheckCircle2 className="size-10 text-ok" />
            <div>
              <h1 className="text-lg font-semibold">Licença ativada</h1>
              <p className="mt-1 text-[13px] text-muted">Redirecionando para o painel…</p>
            </div>
          </>
        ) : null}
        {phase === 'error' ? (
          <>
            <XCircle className="size-10 text-danger" />
            <div>
              <h1 className="text-lg font-semibold">Não foi possível ativar</h1>
              <p className="mt-1 text-[13px] text-muted">{message}</p>
            </div>
            <div className="flex gap-2 pt-1">
              <Button onClick={() => navigate('/manager/login', { replace: true })}>Voltar ao login</Button>
              {code && apiKey ? (
                <Button variant="primary" onClick={() => void run()}>
                  Tentar novamente
                </Button>
              ) : null}
            </div>
          </>
        ) : null}
      </div>
    </AuthLayout>
  );
}
