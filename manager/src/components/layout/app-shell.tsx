import { Outlet } from 'react-router-dom';
import { Menu } from 'lucide-react';
import { useUi } from '@/stores/ui';
import { Button } from '@/components/ui/button';
import { Logo } from '@/components/ui/logo';
import { Sidebar } from './sidebar';

export function AppShell() {
  const setMobileNav = useUi((s) => s.setMobileNav);
  return (
    <div className="flex min-h-dvh">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="sticky top-0 z-30 flex h-12 items-center gap-2 border-b border-line bg-surface/90 px-3 backdrop-blur lg:hidden">
          <Button variant="ghost" size="icon" onClick={() => setMobileNav(true)} aria-label="Abrir menu">
            <Menu className="size-5" />
          </Button>
          <Logo />
        </div>
        <main id="main" className="min-w-0 flex-1">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
