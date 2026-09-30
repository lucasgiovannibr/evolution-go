import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { ExternalLink, KeyRound, LogOut, MoreHorizontal, PlugZap, Send, Settings2, Trash2, Unplug } from 'lucide-react';
import type { Instance } from '@/api/types';
import { copyText } from '@/lib/clipboard';
import { Button } from '@/components/ui/button';
import { Menu, MenuItem, MenuSeparator } from '@/components/ui/menu';
import { useInstanceActions } from './actions';

/** The "more" menu shared by cards, table rows and the detail header. */
export function InstanceMenu({ instance, variant = 'ghost' }: { instance: Instance; variant?: 'ghost' | 'secondary' }) {
  const navigate = useNavigate();
  const actions = useInstanceActions();
  const base = `/manager/instances/${instance.id}`;

  return (
    <Menu
      trigger={(p) => (
        <Button {...p} variant={variant} size="icon" aria-label={`Mais ações para ${instance.name}`}>
          <MoreHorizontal className="size-4" />
        </Button>
      )}
    >
      <MenuItem icon={<ExternalLink />} onSelect={() => navigate(base)}>
        Abrir detalhes
      </MenuItem>
      <MenuItem icon={<Settings2 />} onSelect={() => navigate(`${base}/webhook`)}>
        Webhook e eventos
      </MenuItem>
      <MenuItem icon={<Send />} onSelect={() => navigate(`${base}/test`)}>
        Enviar mensagem de teste
      </MenuItem>
      <MenuItem
        icon={<KeyRound />}
        onSelect={async () => ((await copyText(instance.token)) ? toast.success('Token copiado') : toast.error('Não foi possível copiar'))}
      >
        Copiar token
      </MenuItem>
      <MenuSeparator />
      {instance.connected ? (
        <>
          <MenuItem icon={<Unplug />} onSelect={() => actions.disconnect(instance)}>
            Desconectar
          </MenuItem>
          <MenuItem icon={<LogOut />} onSelect={() => actions.logout(instance)}>
            Desvincular do WhatsApp
          </MenuItem>
        </>
      ) : (
        <MenuItem icon={<PlugZap />} onSelect={() => actions.connect(instance)}>
          Conectar
        </MenuItem>
      )}
      <MenuItem icon={<Trash2 />} danger onSelect={() => actions.remove(instance)}>
        Excluir instância
      </MenuItem>
    </Menu>
  );
}
