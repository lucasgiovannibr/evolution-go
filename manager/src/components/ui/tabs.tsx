import type { ReactNode } from 'react';
import { NavLink } from 'react-router-dom';
import { cn } from '@/lib/cn';

export interface TabItem {
  to: string;
  label: string;
  icon?: ReactNode;
  end?: boolean;
  badge?: ReactNode;
}

/** Route-driven underline tabs: the URL is the source of truth, so tabs are linkable and survive reloads. */
export function TabLinks({ items, label }: { items: TabItem[]; label: string }) {
  return (
    <nav aria-label={label} className="-mb-px flex gap-1 overflow-x-auto">
      {items.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          end={t.end}
          replace
          className={({ isActive }) =>
            cn(
              'relative inline-flex h-10 items-center gap-2 border-b-2 px-3 text-[13px] font-medium whitespace-nowrap transition-colors [&_svg]:size-4',
              isActive ? 'border-brand text-fg [&_svg]:text-brand-text' : 'border-transparent text-muted hover:text-fg',
            )
          }
        >
          {t.icon}
          {t.label}
          {t.badge}
        </NavLink>
      ))}
    </nav>
  );
}
