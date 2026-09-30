import { Link } from 'react-router-dom';
import { Compass } from 'lucide-react';
import { EmptyState } from '@/components/ui/feedback';
import { buttonStyles } from '@/components/ui/button';

export function NotFoundPage() {
  return (
    <div className="flex min-h-[70dvh] items-center justify-center">
      <EmptyState
        icon={<Compass />}
        title="Página não encontrada"
        description="O endereço que você tentou abrir não existe neste painel."
        action={
          <Link to="/manager" className={buttonStyles('primary')}>
            Ir para a visão geral
          </Link>
        }
      />
    </div>
  );
}
