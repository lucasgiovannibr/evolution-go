import { CheckCheck, CircleDashed, PhoneOff, Users, Wifi } from 'lucide-react';
import type { BehaviorSettings } from '@/api/types';
import { useDraft } from '@/hooks/use-draft';
import { useSaveBehavior } from '@/hooks/use-instances';
import { Card, CardBody, CardHeader } from '@/components/ui/card';
import { Field, Textarea } from '@/components/ui/form';
import { SaveBar } from '@/components/ui/save-bar';
import { SwitchRow } from '@/components/ui/switch';
import { useInstanceContext } from './instance-page';

export function TabBehavior() {
  const { instance } = useInstanceContext();
  const save = useSaveBehavior(instance);
  const { draft, patch, dirty, reset } = useDraft<BehaviorSettings>(instance.behavior);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader title="Comportamento" description="Como a instância se apresenta e o que ela processa." />
        <CardBody className="divide-y divide-line py-1">
          <SwitchRow
            icon={<Wifi />}
            title="Sempre online"
            description="Mantém a presença como online mesmo sem atividade."
            checked={draft.alwaysOnline}
            onChange={(v) => patch({ alwaysOnline: v })}
          />
          <div>
            <SwitchRow
              icon={<PhoneOff />}
              title="Rejeitar chamadas"
              description="Recusa automaticamente chamadas de voz e vídeo."
              checked={draft.rejectCall}
              onChange={(v) => patch({ rejectCall: v })}
            />
            {draft.rejectCall ? (
              <div className="pb-3 pl-11">
                <Field label="Mensagem enviada ao rejeitar" optional hint="Se preenchida, quem ligou recebe este texto logo após a chamada ser recusada.">
                  {(id) => (
                    <Textarea
                      id={id}
                      rows={2}
                      value={draft.msgRejectCall}
                      onChange={(e) => patch({ msgRejectCall: e.target.value })}
                      placeholder="Não atendemos chamadas por aqui. Envie uma mensagem, por favor."
                    />
                  )}
                </Field>
              </div>
            ) : null}
          </div>
          <SwitchRow
            icon={<CheckCheck />}
            title="Marcar como lidas"
            description="Marca as mensagens recebidas como lidas automaticamente."
            checked={draft.readMessages}
            onChange={(v) => patch({ readMessages: v })}
          />
          <SwitchRow
            icon={<Users />}
            title="Ignorar grupos"
            description="Não processa mensagens vindas de grupos."
            checked={draft.ignoreGroups}
            onChange={(v) => patch({ ignoreGroups: v })}
          />
          <SwitchRow
            icon={<CircleDashed />}
            title="Ignorar status"
            description="Não processa atualizações de status (stories)."
            checked={draft.ignoreStatus}
            onChange={(v) => patch({ ignoreStatus: v })}
          />
        </CardBody>
      </Card>

      <SaveBar dirty={dirty} saving={save.isPending} onSave={() => save.mutate(draft)} onReset={reset} />
    </div>
  );
}
