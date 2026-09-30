import type { SendKind } from '@/api/messages';

export type PresetGroup = Exclude<SendKind, 'text'>;

export interface Preset {
  id: string;
  group: PresetGroup;
  label: string;
  description: string;
  build: (number: string) => Record<string, unknown>;
}

const FOOTER = 'Evolution GO';
const plus = (n: string) => `+${n.replace(/\D/g, '')}`;
const img = (seed: string) => ({ imageUrl: `https://picsum.photos/seed/${seed}/600/400` });

const button = (title: string, description: string, buttons: unknown[]) => (number: string) => ({
  number,
  title,
  description,
  footer: FOOTER,
  buttons,
});

const carousel = (body: string, cards: unknown[]) => (number: string) => ({ number, body, footer: FOOTER, cards });

/** Ready-made payloads for the interactive endpoints, kept in sync with the server validation rules. */
export const PRESETS: Preset[] = [
  {
    id: 'btn_reply_1',
    group: 'button',
    label: 'Reply (1 botão)',
    description: 'Um botão do tipo reply (quick reply).',
    build: button('Teste - Reply único', 'Um botão do tipo reply.', [{ type: 'reply', displayText: 'Confirmar', id: 'test_reply_1' }]),
  },
  {
    id: 'btn_reply_3',
    group: 'button',
    label: 'Reply (3 botões, limite)',
    description: 'Três botões reply: o máximo aceito pelo servidor.',
    build: button('Teste - 3 Reply (limite)', 'Três botões reply.', [
      { type: 'reply', displayText: 'Opção A', id: 'test_a' },
      { type: 'reply', displayText: 'Opção B', id: 'test_b' },
      { type: 'reply', displayText: 'Opção C', id: 'test_c' },
    ]),
  },
  {
    id: 'btn_copy',
    group: 'button',
    label: 'CTA Copiar',
    description: 'Botão que copia um código para a área de transferência.',
    build: button('Teste - CTA Copy', 'Botão COPY com código a ser copiado.', [{ type: 'copy', displayText: 'Copiar cupom', copyCode: 'PROMO2026' }]),
  },
  {
    id: 'btn_url',
    group: 'button',
    label: 'CTA URL',
    description: 'Botão que abre um link no navegador.',
    build: button('Teste - CTA URL', 'Botão que abre um link.', [{ type: 'url', displayText: 'Abrir site', url: 'https://evolutionapi.com' }]),
  },
  {
    id: 'btn_call',
    group: 'button',
    label: 'CTA Ligar',
    description: 'Botão que inicia uma ligação telefônica.',
    build: (n) => button('Teste - CTA Call', 'Botão que inicia uma ligação.', [{ type: 'call', displayText: 'Ligar agora', phoneNumber: plus(n) }])(n),
  },
  {
    id: 'btn_pix',
    group: 'button',
    label: 'PIX (sozinho)',
    description: 'Pagamento Pix. O servidor exige que seja o único botão.',
    build: button('Teste - PIX', 'Botão Pix (envia sozinho).', [{ type: 'pix', currency: 'BRL', name: 'Minha Loja', keyType: 'cpf', key: '12345678900' }]),
  },
  {
    id: 'btn_cta_group',
    group: 'button',
    label: 'CTAs agrupados',
    description: 'Copiar + URL + Ligar. Combinação recomendada para o WhatsApp Web; não mistura com reply.',
    build: (n) =>
      button('Teste - CTAs agrupados', 'copy + url + call (funciona no WhatsApp Web).', [
        { type: 'copy', displayText: 'Copiar cupom', copyCode: 'CTA2026' },
        { type: 'url', displayText: 'Abrir site', url: 'https://evolutionapi.com' },
        { type: 'call', displayText: 'Ligar agora', phoneNumber: plus(n) },
      ])(n),
  },
  {
    id: 'list',
    group: 'list',
    label: 'Lista com seções',
    description: 'Menu de seleção única com duas seções (Planos e Suporte).',
    build: (number) => ({
      number,
      title: 'Teste - Lista',
      description: 'Lista interativa com seções e itens.',
      buttonText: 'Ver opções',
      footerText: FOOTER,
      sections: [
        {
          title: 'Planos',
          rows: [
            { title: 'Plano Básico', description: 'R$ 29,90/mês', rowId: 'plan_basic' },
            { title: 'Plano Pro', description: 'R$ 59,90/mês', rowId: 'plan_pro' },
          ],
        },
        {
          title: 'Suporte',
          rows: [
            { title: 'Falar com atendente', description: 'Horário comercial', rowId: 'support_agent' },
            { title: 'Central de ajuda', description: 'Artigos e FAQ', rowId: 'support_kb' },
          ],
        },
      ],
    }),
  },
  {
    id: 'carousel_reply',
    group: 'carousel',
    label: 'Carrossel com REPLY',
    description: '4 cards (Básico, Pro, Business, Enterprise) com botões do tipo REPLY.',
    build: carousel('Teste - Carrossel com botões REPLY', [
      { header: img('replyA'), body: { text: 'Card A - Plano Básico' }, footer: 'R$ 29,90 / mês', buttons: [{ type: 'REPLY', displayText: 'Assinar Básico', id: 'reply_basic' }, { type: 'REPLY', displayText: 'Saber mais', id: 'reply_basic_info' }] },
      { header: img('replyB'), body: { text: 'Card B - Plano Pro' }, footer: 'R$ 59,90 / mês', buttons: [{ type: 'REPLY', displayText: 'Assinar Pro', id: 'reply_pro' }, { type: 'REPLY', displayText: 'Saber mais', id: 'reply_pro_info' }] },
      { header: img('replyC'), body: { text: 'Card C - Plano Business' }, footer: 'R$ 149,90 / mês', buttons: [{ type: 'REPLY', displayText: 'Assinar Business', id: 'reply_business' }, { type: 'REPLY', displayText: 'Saber mais', id: 'reply_business_info' }] },
      { header: img('replyD'), body: { text: 'Card D - Plano Enterprise' }, footer: 'Sob consulta', buttons: [{ type: 'REPLY', displayText: 'Falar com vendas', id: 'reply_enterprise' }] },
    ]),
  },
  {
    id: 'carousel_url',
    group: 'carousel',
    label: 'Carrossel com URL',
    description: '4 cards (Site, Docs, GitHub, Comunidade). No carrossel o link vai no campo id.',
    build: carousel('Teste - Carrossel com botão URL', [
      { header: img('urlA'), body: { text: 'Card A - Site oficial' }, footer: 'Abre o site principal', buttons: [{ type: 'URL', displayText: 'Abrir site', id: 'https://evolutionapi.com' }] },
      { header: img('urlB'), body: { text: 'Card B - Documentação' }, footer: 'Abre os docs da API', buttons: [{ type: 'URL', displayText: 'Ver documentação', id: 'https://doc.evolutionapi.com' }] },
      { header: img('urlC'), body: { text: 'Card C - GitHub' }, footer: 'Abre o repositório', buttons: [{ type: 'URL', displayText: 'Abrir GitHub', id: 'https://github.com/evolution-foundation' }] },
      { header: img('urlD'), body: { text: 'Card D - Comunidade' }, footer: 'Participe da comunidade', buttons: [{ type: 'URL', displayText: 'Entrar na comunidade', id: 'https://evolutionapi.com/community' }] },
    ]),
  },
  {
    id: 'carousel_call',
    group: 'carousel',
    label: 'Carrossel com LIGAR',
    description: '3 cards (Atendimento, Suporte, Financeiro). No carrossel o telefone vai no campo id.',
    build: (n) =>
      carousel('Teste - Carrossel com botão CALL', [
        { header: img('callA'), body: { text: 'Card A - Atendimento geral' }, footer: 'Horário comercial', buttons: [{ type: 'CALL', displayText: 'Ligar - Atendimento', id: plus(n) }] },
        { header: img('callB'), body: { text: 'Card B - Suporte técnico' }, footer: '24x7', buttons: [{ type: 'CALL', displayText: 'Ligar - Suporte', id: plus(n) }] },
        { header: img('callC'), body: { text: 'Card C - Financeiro' }, footer: 'Seg a Sex, 9h-18h', buttons: [{ type: 'CALL', displayText: 'Ligar - Financeiro', id: plus(n) }] },
      ])(n),
  },
  {
    id: 'carousel_copy',
    group: 'carousel',
    label: 'Carrossel com COPIAR',
    description: '4 cards com cupons distintos usando o campo copyCode.',
    build: carousel('Teste - Carrossel com botão COPY', [
      { header: img('copyA'), body: { text: 'Card A - Cupom de primeira compra' }, footer: '10% de desconto', buttons: [{ type: 'COPY', displayText: 'Copiar cupom', copyCode: 'BEMVINDO10' }] },
      { header: img('copyB'), body: { text: 'Card B - Cupom Black Friday' }, footer: '30% de desconto', buttons: [{ type: 'COPY', displayText: 'Copiar cupom', copyCode: 'BLACK30' }] },
      { header: img('copyC'), body: { text: 'Card C - Cupom anual' }, footer: '2 meses grátis', buttons: [{ type: 'COPY', displayText: 'Copiar cupom', copyCode: 'ANUAL2MESES' }] },
      { header: img('copyD'), body: { text: 'Card D - Cupom VIP' }, footer: 'Exclusivo para clientes', buttons: [{ type: 'COPY', displayText: 'Copiar cupom VIP', copyCode: 'VIP2026' }] },
    ]),
  },
];

export const presetsOf = (group: PresetGroup) => PRESETS.filter((p) => p.group === group);
