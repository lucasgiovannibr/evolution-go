import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import type { Instance } from '@/api/types';
import { errMsg, useDeleteInstance, useDisconnectInstance, useLogoutInstance } from '@/hooks/use-instances';
import { ConfirmDialog } from '@/components/ui/dialog';
import { ConnectDialog } from './connect-dialog';
import { CreateInstanceDialog } from './create-instance-dialog';

interface InstanceActions {
  create: () => void;
  connect: (i: Pick<Instance, 'id'>) => void;
  disconnect: (i: Instance) => void;
  logout: (i: Instance) => void;
  remove: (i: Instance) => void;
}

const Ctx = createContext<InstanceActions | null>(null);

export function useInstanceActions(): InstanceActions {
  const v = useContext(Ctx);
  if (!v) throw new Error('useInstanceActions must be used inside <InstanceActionsProvider>');
  return v;
}

/**
 * Owns every instance-level dialog once, so cards, rows, the detail page and the overview
 * all trigger the same flows without each mounting their own copy.
 */
export function InstanceActionsProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const { pathname } = useLocation();

  const [creating, setCreating] = useState(false);
  const [connectId, setConnectId] = useState<string | null>(null);
  const [toDisconnect, setToDisconnect] = useState<Instance | null>(null);
  const [toLogout, setToLogout] = useState<Instance | null>(null);
  const [toRemove, setToRemove] = useState<Instance | null>(null);

  const disconnect = useDisconnectInstance();
  const logout = useLogoutInstance();
  const remove = useDeleteInstance();

  const actions = useMemo<InstanceActions>(
    () => ({
      create: () => setCreating(true),
      connect: (i) => setConnectId(i.id),
      disconnect: setToDisconnect,
      logout: setToLogout,
      remove: setToRemove,
    }),
    [],
  );

  const confirmDisconnect = useCallback(async () => {
    if (!toDisconnect) return;
    try {
      await disconnect.mutateAsync(toDisconnect);
      toast.success(`${toDisconnect.name} desconectada`);
      setToDisconnect(null);
    } catch (e) {
      toast.error('Não foi possível desconectar', { description: errMsg(e) });
    }
  }, [toDisconnect, disconnect]);

  const confirmLogout = useCallback(async () => {
    if (!toLogout) return;
    try {
      await logout.mutateAsync(toLogout);
      toast.success(`${toLogout.name} desvinculada do WhatsApp`);
      setToLogout(null);
    } catch (e) {
      toast.error('Não foi possível desvincular', { description: errMsg(e) });
    }
  }, [toLogout, logout]);

  const confirmRemove = useCallback(async () => {
    if (!toRemove) return;
    try {
      await remove.mutateAsync(toRemove);
      toast.success(`Instância ${toRemove.name} excluída`);
      setToRemove(null);
      if (pathname.startsWith('/manager/instances/')) navigate('/manager/instances', { replace: true });
    } catch (e) {
      toast.error('Não foi possível excluir', { description: errMsg(e) });
    }
  }, [toRemove, remove, pathname, navigate]);

  return (
    <Ctx.Provider value={actions}>
      {children}

      <CreateInstanceDialog
        open={creating}
        onClose={() => setCreating(false)}
        onCreated={(created) => {
          setCreating(false);
          toast.success(`Instância ${created.name} criada`);
          navigate(`/manager/instances/${created.id}`);
          setConnectId(created.id);
        }}
      />

      <ConnectDialog instanceId={connectId} onClose={() => setConnectId(null)} />

      <ConfirmDialog
        open={!!toDisconnect}
        onClose={() => setToDisconnect(null)}
        onConfirm={() => void confirmDisconnect()}
        loading={disconnect.isPending}
        tone="primary"
        title="Desconectar instância?"
        description={
          <>
            A conexão de <strong className="font-medium text-fg">{toDisconnect?.name}</strong> será encerrada, mas o número continua vinculado: dá para reconectar sem ler um novo QR Code.
          </>
        }
        confirmLabel="Desconectar"
      />

      <ConfirmDialog
        open={!!toLogout}
        onClose={() => setToLogout(null)}
        onConfirm={() => void confirmLogout()}
        loading={logout.isPending}
        title="Desvincular do WhatsApp?"
        description={
          <>
            <strong className="font-medium text-fg">{toLogout?.name}</strong> sairá da conta do WhatsApp e será removida dos aparelhos conectados. Para usar de novo será preciso ler um novo QR Code.
          </>
        }
        confirmLabel="Desvincular"
      />

      <ConfirmDialog
        open={!!toRemove}
        onClose={() => setToRemove(null)}
        onConfirm={() => void confirmRemove()}
        loading={remove.isPending}
        title="Excluir instância?"
        description="Esta ação é permanente: a instância, suas configurações e a sessão do WhatsApp serão removidas."
        confirmLabel="Excluir definitivamente"
        requireText={toRemove?.name}
      />
    </Ctx.Provider>
  );
}
