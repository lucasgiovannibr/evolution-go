import { useEffect, type ReactNode } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { BookOpen, ExternalLink, FlaskConical, LayoutDashboard, LogOut, Moon, PanelLeftClose, PanelLeftOpen, Smartphone, Sun, X } from 'lucide-react';
import { cn } from '@/lib/cn';
import { hostOf } from '@/lib/format';
import { useAuth } from '@/stores/auth';
import { useUi } from '@/stores/ui';
import { HEALTH_META, useHealth } from '@/hooks/use-health';
import { Logo } from '@/components/ui/logo';
import { Button } from '@/components/ui/button';

interface NavItem {
  to: string;
  label: string;
  icon: ReactNode;
  end?: boolean;
}

const NAV: NavItem[] = [
  { to: '/manager', label: 'Visão geral', icon: <LayoutDashboard />, end: true },
  { to: '/manager/instances', label: 'Instâncias', icon: <Smartphone /> },
  { to: '/manager/api-tester', label: 'Explorador da API', icon: <FlaskConical /> },
];

function NavRow({ item, collapsed }: { item: NavItem; collapsed: boolean }) {
  return (
    <NavLink
      to={item.to}
      end={item.end}
      title={collapsed ? item.label : undefined}
      className={({ isActive }) =>
        cn(
          'group relative flex h-9 items-center gap-2.5 rounded-control px-2.5 text-[13px] font-medium transition-colors [&_svg]:size-[18px] [&_svg]:shrink-0',
          collapsed && 'justify-center px-0',
          isActive ? 'bg-brand-soft text-brand-text' : 'text-muted hover:bg-surface-2 hover:text-fg',
        )
      }
    >
      {({ isActive }) => (
        <>
          {isActive ? <span className="absolute top-2 bottom-2 -left-3 w-[3px] rounded-r-full bg-brand" aria-hidden /> : null}
          {item.icon}
          {collapsed ? <span className="sr-only">{item.label}</span> : <span className="truncate">{item.label}</span>}
        </>
      )}
    </NavLink>
  );
}

function ServerCard({ collapsed }: { collapsed: boolean }) {
  const apiUrl = useAuth((s) => s.apiUrl);
  const { data: health = 'ok', isPending } = useHealth();
  const meta = HEALTH_META[health];
  const dot = meta.tone === 'ok' ? 'bg-ok' : meta.tone === 'warn' ? 'bg-warn' : 'bg-danger';

  if (collapsed) {
    return (
      <div className="flex justify-center py-2" title={`${hostOf(apiUrl)} · ${meta.label}`}>
        <span className={cn('size-2 rounded-full', isPending ? 'bg-subtle' : dot, health === 'ok' && 'animate-pulse-ring')} />
      </div>
    );
  }
  return (
    <div className="rounded-control border border-line bg-surface-2/60 px-3 py-2" title={meta.detail}>
      <div className="flex items-center gap-2 text-[11px] font-medium tracking-wide text-subtle uppercase">
        <span className={cn('size-1.5 rounded-full', isPending ? 'bg-subtle' : dot, health === 'ok' && 'animate-pulse-ring')} />
        {isPending ? 'Verificando…' : meta.label}
      </div>
      <p className="mt-0.5 truncate font-mono text-xs text-fg">{hostOf(apiUrl)}</p>
    </div>
  );
}

interface SidebarBodyProps {
  collapsed: boolean;
  onNavigate?: () => void;
  mobile?: boolean;
}

function SidebarBody({ collapsed, onNavigate, mobile }: SidebarBodyProps) {
  const { theme, toggleTheme, toggleSidebar, setMobileNav } = useUi();
  const clear = useAuth((s) => s.clear);
  const apiUrl = useAuth((s) => s.apiUrl);
  const swagger = `${apiUrl.replace(/\/$/, '')}/swagger/index.html`;

  const logout = () => {
    clear();
    window.location.assign('/manager/login');
  };

  return (
    <div className="flex h-full flex-col">
      <div className={cn('flex h-14 shrink-0 items-center px-4', collapsed && 'justify-center px-0')}>
        <Logo compact={collapsed} />
        {mobile ? (
          <Button variant="ghost" size="icon-sm" className="ml-auto" onClick={() => setMobileNav(false)} aria-label="Fechar menu">
            <X className="size-4" />
          </Button>
        ) : null}
      </div>

      <nav aria-label="Principal" className={cn('flex-1 space-y-0.5 overflow-y-auto px-3 py-2', collapsed && 'px-2')} onClick={onNavigate}>
        {NAV.map((item) => (
          <NavRow key={item.to} item={item} collapsed={collapsed} />
        ))}
        <a
          href={swagger}
          target="_blank"
          rel="noreferrer noopener"
          title={collapsed ? 'Swagger' : undefined}
          className={cn(
            'flex h-9 items-center gap-2.5 rounded-control px-2.5 text-[13px] font-medium text-muted transition-colors hover:bg-surface-2 hover:text-fg [&_svg]:size-[18px] [&_svg]:shrink-0',
            collapsed && 'justify-center px-0',
          )}
        >
          <BookOpen />
          {collapsed ? <span className="sr-only">Swagger</span> : <span className="flex-1 truncate">Swagger</span>}
          {collapsed ? null : <ExternalLink className="!size-3.5 text-subtle" />}
        </a>
      </nav>

      <div className={cn('shrink-0 space-y-2 border-t border-line p-3', collapsed && 'px-2')}>
        <ServerCard collapsed={collapsed} />
        <div className={cn('flex items-center gap-1', collapsed && 'flex-col')}>
          <Button variant="ghost" size="icon" onClick={toggleTheme} aria-label={theme === 'dark' ? 'Usar tema claro' : 'Usar tema escuro'} title={theme === 'dark' ? 'Tema claro' : 'Tema escuro'}>
            {theme === 'dark' ? <Sun className="size-4" /> : <Moon className="size-4" />}
          </Button>
          {mobile ? null : (
            <Button variant="ghost" size="icon" onClick={toggleSidebar} aria-label={collapsed ? 'Expandir menu' : 'Recolher menu'} title={collapsed ? 'Expandir menu' : 'Recolher menu'} className="max-lg:hidden">
              {collapsed ? <PanelLeftOpen className="size-4" /> : <PanelLeftClose className="size-4" />}
            </Button>
          )}
          <Button variant="ghost" size={collapsed ? 'icon' : 'md'} onClick={logout} className={cn(!collapsed && 'ml-auto text-muted')} aria-label="Sair" title="Sair">
            <LogOut className="size-4" />
            {collapsed ? null : 'Sair'}
          </Button>
        </div>
      </div>
    </div>
  );
}

export function Sidebar() {
  const collapsed = useUi((s) => s.sidebarCollapsed);
  const mobileOpen = useUi((s) => s.mobileNavOpen);
  const setMobileNav = useUi((s) => s.setMobileNav);
  const { pathname } = useLocation();

  useEffect(() => setMobileNav(false), [pathname, setMobileNav]);
  useEffect(() => {
    if (!mobileOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setMobileNav(false);
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [mobileOpen, setMobileNav]);

  return (
    <>
      <aside
        className={cn(
          'sticky top-0 hidden h-dvh shrink-0 border-r border-line bg-surface transition-[width] duration-200 lg:block',
          collapsed ? 'w-16' : 'w-60',
        )}
      >
        <SidebarBody collapsed={collapsed} />
      </aside>

      {mobileOpen ? (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div className="absolute inset-0 animate-fade-in bg-(--overlay)" onClick={() => setMobileNav(false)} aria-hidden />
          <aside className="absolute inset-y-0 left-0 w-72 max-w-[85vw] animate-slide-in border-r border-line bg-surface shadow-pop">
            <SidebarBody collapsed={false} mobile />
          </aside>
        </div>
      ) : null}
    </>
  );
}
