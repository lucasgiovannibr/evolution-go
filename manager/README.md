# Evolution GO Manager

Painel web da Evolution GO. React 19 + TypeScript + Vite + Tailwind CSS v4.

O servidor Go entrega o resultado do build: `manager/dist/index.html` para `/manager/*` e os
arquivos com hash em `manager/dist/assets` (ver `pkg/routes/routes.go`). O `Dockerfile` copia
`manager/dist` do repositório, então **o `dist` é versionado e precisa ser regenerado a cada mudança no painel**.

## Desenvolvimento

```bash
cd manager
npm install
npm run dev          # http://localhost:5173/manager  (proxy para VITE_API_TARGET, padrão http://localhost:8080)
npm test             # vitest (lógica pura em src/lib e src/features/*)
npm run build        # typecheck + build em manager/dist
```

Para apontar o `dev` para outro servidor: `VITE_API_TARGET=http://meu-host:8080 npm run dev`.

## Estrutura

```
src/
  api/          chamadas tipadas à API (instâncias, sessão/licença, mensagens) e o modelo de domínio
  components/
    ui/         primitivos (Button, Dialog, Menu, Switch, Tabs, Badge...), sem regra de negócio
    layout/     AppShell, Sidebar, PageHeader
  features/     uma pasta por área: auth, overview, instances, instance-detail, explorer
  hooks/        React Query (instâncias, saúde), rascunho de formulário
  lib/          http, formatação, catálogo de eventos, cURL
  stores/       zustand: sessão (persistida em "evolution-auth") e UI (tema, menu)
  styles.css    tokens de design (cores, sombras, animações) e tema claro/escuro
```

## Convenções

- **Cores só por token** (`bg-surface`, `text-muted`, `border-line`, `bg-brand`...), nunca hex nos componentes:
  os tokens já cobrem tema claro e escuro.
- **Diálogos e menus** usam portal próprio (não `<dialog>`), para os toasts ficarem sempre acima.
- **Catálogo de eventos** em `src/lib/events.ts` espelha `pkg/internal/event_types` no servidor; ao criar um
  evento novo no Go, inclua-o aqui.
- **Estado de servidor** vive no React Query; formulários editam uma cópia (`useDraft`) que só é
  substituída por dados novos quando não há edição pendente.
- O login preserva o fluxo de licença (`/license/status` → `/license/register` → `/manager/license/callback`
  → `/license/activate`) e a chave `evolution-auth` do `localStorage`, para sessões já abertas continuarem válidas.

As fontes (Inter e JetBrains Mono, licença OFL) ficam em `src/assets/fonts`.
