import { presetsOf, type PresetGroup } from './presets';

/** Every message the public API can send to a chat or to the status (path after /send/). */
export type SendKind =
  | 'text'
  | 'link'
  | 'location'
  | 'contact'
  | 'poll'
  | 'media'
  | 'sticker'
  | 'button'
  | 'list'
  | 'carousel'
  | 'status/text'
  | 'status/media';

export type KindGroup = 'basic' | 'media' | 'interactive' | 'status';

export const GROUPS: { id: KindGroup; label: string }[] = [
  { id: 'basic', label: 'Básicas' },
  { id: 'media', label: 'Mídia' },
  { id: 'interactive', label: 'Interativas' },
  { id: 'status', label: 'Status' },
];

export type Values = Record<string, string>;

export interface FieldDef {
  key: string;
  label: string;
  control: 'input' | 'textarea' | 'select';
  options?: { value: string; label: string }[];
  placeholder?: string;
  hint?: string;
  optional?: boolean;
  rows?: number;
  inputMode?: 'numeric' | 'decimal' | 'url' | 'tel';
  /** Hides the field while it does not apply to the current values. */
  showIf?: (v: Values) => boolean;
}

export interface KindDef {
  id: SendKind;
  label: string;
  group: KindGroup;
  description: string;
  fields: FieldDef[];
  defaults: Values;
  /** False for the status, which goes to the account's own status and has no recipient. */
  recipient: boolean;
  note?: { tone: 'info' | 'warn'; text: string };
  /** Text of a checkbox the person has to tick: the message reaches people beyond the test number. */
  confirm?: string;
  build: (number: string, v: Values) => Record<string, unknown>;
  /** Returns what is wrong with the values, or null. */
  validate: (v: Values) => string | null;
  /** Values after `key` changed (e.g. a sample URL that follows the media type). */
  patch?: (key: string, value: string, v: Values) => Values;
}

export const SAMPLE = {
  image: 'https://picsum.photos/seed/evolution/800/600.jpg',
  video: 'https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4',
  audio: 'https://interactive-examples.mdn.mozilla.net/media/cc0-audio/t-rex-roar.mp3',
  document: 'https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf',
  sticker: 'https://www.gstatic.com/webp/gallery/1.webp',
} as const;

const MEDIA_TYPES = [
  { value: 'image', label: 'Imagem' },
  { value: 'video', label: 'Vídeo' },
  { value: 'audio', label: 'Áudio' },
  { value: 'document', label: 'Documento' },
];

const trimmed = (v: string | undefined) => (v ?? '').trim();
const required = (v: Values, rules: [string, string][]) => {
  for (const [key, label] of rules) if (!trimmed(v[key])) return `Preencha ${label}.`;
  return null;
};
const isHttp = (s: string) => /^https?:\/\/\S+$/i.test(s.trim());
const urlError = (s: string, label: string) => (isHttp(s) ? null : `${label} precisa ser um endereço http(s) completo.`);
/** Adds the optional text fields only when they have content, so the preview matches what goes out. */
const withText = (base: Record<string, unknown>, extra: Record<string, string | undefined>) => {
  for (const [k, val] of Object.entries(extra)) if (trimmed(val)) base[k] = trimmed(val);
  return base;
};
const coord = (s: string, min: number, max: number) => {
  const n = Number(s.replace(',', '.'));
  return Number.isFinite(n) && n >= min && n <= max ? n : null;
};
const pollOptions = (s: string) =>
  s
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);

/** Interactive messages are built from ready-made payloads, kept in presets.ts. */
function presetKind(
  group: PresetGroup,
  def: Pick<KindDef, 'id' | 'label' | 'description' | 'note'> & { first: string },
): KindDef {
  return {
    ...def,
    group: 'interactive',
    recipient: true,
    fields: [
      {
        key: 'preset',
        label: 'Modelo',
        control: 'select',
        options: presetsOf(group).map((p) => ({ value: p.id, label: p.label })),
      },
    ],
    defaults: { preset: def.first },
    build: (number, v) => presetsOf(group).find((p) => p.id === v.preset)?.build(number) ?? {},
    validate: () => null,
  };
}

/** The list is refused by WhatsApp on linked devices; the server then sends it as reply buttons unless told not to. */
function listKind(): KindDef {
  const base = presetKind('list', {
    id: 'list',
    label: 'Lista',
    description: 'Menu de seleção única com seções.',
    first: 'list_short',
    note: {
      tone: 'warn',
      text: 'O WhatsApp recusa listas em sessões de aparelho vinculado, mesmo em contas Business. Por isso o servidor envia a lista como botões de resposta (até 9 itens, 3 por mensagem); a resposta traz Fallback: "buttons".',
    },
  });
  return {
    ...base,
    fields: [
      ...base.fields,
      {
        key: 'onRefusal',
        label: 'Se o WhatsApp recusar a lista',
        control: 'select',
        options: [
          { value: 'buttons', label: 'Enviar como botões de resposta' },
          { value: 'error', label: 'Devolver o erro (502)' },
        ],
      },
    ],
    defaults: { ...base.defaults, onRefusal: 'buttons' },
    build: (number, v) => {
      const body = base.build(number, v);
      if (v.onRefusal === 'error') body.fallbackButtons = false;
      return body;
    },
  };
}

export const KINDS: KindDef[] = [
  {
    id: 'text',
    label: 'Texto',
    group: 'basic',
    description: 'Mensagem de texto simples. Aceita *negrito*, _itálico_ e ~riscado~.',
    recipient: true,
    fields: [{ key: 'text', label: 'Mensagem', control: 'textarea', rows: 4, placeholder: 'Digite sua mensagem…' }],
    defaults: { text: '' },
    build: (number, v) => ({ number, text: v.text }),
    validate: (v) => required(v, [['text', 'a mensagem']]),
  },
  {
    id: 'link',
    label: 'Link',
    group: 'basic',
    description: 'Texto com pré-visualização de um link (título, descrição e imagem).',
    recipient: true,
    fields: [
      { key: 'url', label: 'Link', control: 'input', inputMode: 'url', placeholder: 'https://evolutionapi.com' },
      { key: 'text', label: 'Texto', control: 'textarea', rows: 2, hint: 'Vai acima do cartão do link.' },
      { key: 'title', label: 'Título do cartão', control: 'input', optional: true },
      { key: 'description', label: 'Descrição do cartão', control: 'input', optional: true },
      { key: 'imgUrl', label: 'Imagem do cartão (URL)', control: 'input', inputMode: 'url', optional: true },
    ],
    defaults: { url: 'https://evolutionapi.com', text: 'Dê uma olhada neste link', title: 'Evolution API', description: 'Teste de envio de link', imgUrl: SAMPLE.image },
    build: (number, v) => withText({ number, url: trimmed(v.url), text: v.text }, { title: v.title, description: v.description, imgUrl: v.imgUrl }),
    validate: (v) => urlError(v.url ?? '', 'O link') ?? required(v, [['text', 'o texto']]) ?? (trimmed(v.imgUrl) ? urlError(v.imgUrl ?? '', 'A imagem') : null),
  },
  {
    id: 'location',
    label: 'Localização',
    group: 'basic',
    description: 'Pino de localização no mapa.',
    recipient: true,
    fields: [
      { key: 'name', label: 'Nome do local', control: 'input' },
      { key: 'address', label: 'Endereço', control: 'input', optional: true },
      { key: 'latitude', label: 'Latitude', control: 'input', inputMode: 'decimal', hint: 'Entre -90 e 90.' },
      { key: 'longitude', label: 'Longitude', control: 'input', inputMode: 'decimal', hint: 'Entre -180 e 180.' },
    ],
    defaults: { name: 'Praça da Liberdade', address: 'Belo Horizonte, MG', latitude: '-19.9319', longitude: '-43.9378' },
    build: (number, v) =>
      withText({ number, name: trimmed(v.name), latitude: coord(v.latitude ?? '', -90, 90) ?? 0, longitude: coord(v.longitude ?? '', -180, 180) ?? 0 }, { address: v.address }),
    validate: (v) =>
      required(v, [['name', 'o nome do local']]) ??
      (coord(v.latitude ?? '', -90, 90) === null ? 'A latitude precisa estar entre -90 e 90.' : null) ??
      (coord(v.longitude ?? '', -180, 180) === null ? 'A longitude precisa estar entre -180 e 180.' : null),
  },
  {
    id: 'contact',
    label: 'Contato',
    group: 'basic',
    description: 'Cartão de contato (vCard) com nome, empresa e telefone.',
    recipient: true,
    fields: [
      { key: 'fullName', label: 'Nome completo', control: 'input' },
      { key: 'organization', label: 'Empresa', control: 'input', optional: true },
      { key: 'phone', label: 'Telefone', control: 'input', inputMode: 'tel', hint: 'Com DDI, só dígitos.' },
    ],
    defaults: { fullName: 'Contato de Teste', organization: 'Evolution GO', phone: '5511999990000' },
    build: (number, v) => ({
      number,
      vcard: { fullName: trimmed(v.fullName), organization: trimmed(v.organization), phone: (v.phone ?? '').replace(/\D/g, '') },
    }),
    validate: (v) => required(v, [['fullName', 'o nome'], ['phone', 'o telefone']]) ?? ((v.phone ?? '').replace(/\D/g, '').length < 8 ? 'Informe o telefone completo, com DDI.' : null),
  },
  {
    id: 'poll',
    label: 'Enquete',
    group: 'basic',
    description: 'Enquete com 2 a 12 opções; a pessoa vota tocando na opção.',
    recipient: true,
    fields: [
      { key: 'question', label: 'Pergunta', control: 'input' },
      { key: 'options', label: 'Opções', control: 'textarea', rows: 4, hint: 'Uma por linha.' },
      {
        key: 'mode',
        label: 'Respostas',
        control: 'select',
        options: [
          { value: 'single', label: 'Escolha única' },
          { value: 'multi', label: 'Múltipla escolha' },
        ],
      },
    ],
    defaults: { question: 'Qual opção você prefere?', options: 'Opção A\nOpção B\nOpção C', mode: 'single' },
    build: (number, v) => {
      const options = pollOptions(v.options ?? '');
      return { number, question: trimmed(v.question), options, maxAnswer: v.mode === 'multi' ? options.length : 1 };
    },
    validate: (v) => {
      const n = pollOptions(v.options ?? '').length;
      return required(v, [['question', 'a pergunta']]) ?? (n < 2 || n > 12 ? 'A enquete precisa de 2 a 12 opções.' : null);
    },
  },
  {
    id: 'media',
    label: 'Mídia',
    group: 'media',
    description: 'Imagem, vídeo, áudio ou documento a partir de uma URL pública.',
    recipient: true,
    fields: [
      { key: 'type', label: 'Tipo', control: 'select', options: MEDIA_TYPES },
      { key: 'url', label: 'URL do arquivo', control: 'input', inputMode: 'url', hint: 'O servidor baixa o arquivo; precisa ser público.' },
      { key: 'caption', label: 'Legenda', control: 'input', optional: true, showIf: (v) => v.type !== 'audio' },
      { key: 'filename', label: 'Nome do arquivo', control: 'input', optional: true, showIf: (v) => v.type === 'document' },
      {
        key: 'viewOnce',
        label: 'Visualização única',
        control: 'select',
        showIf: (v) => v.type !== 'document',
        options: [
          { value: 'no', label: 'Não' },
          { value: 'yes', label: 'Sim, some depois de aberta' },
        ],
      },
    ],
    defaults: { type: 'image', url: SAMPLE.image, caption: 'Teste de mídia', filename: 'teste.pdf', viewOnce: 'no' },
    build: (number, v) => {
      const payload: Record<string, unknown> = { number, type: v.type, url: trimmed(v.url) };
      if (v.type !== 'audio') withText(payload, { caption: v.caption });
      if (v.type === 'document') withText(payload, { filename: v.filename });
      if (v.type !== 'document' && v.viewOnce === 'yes') payload.viewOnce = true;
      return payload;
    },
    validate: (v) => urlError(v.url ?? '', 'A URL do arquivo'),
    patch: (key, value, v) => {
      if (key !== 'type') return { ...v, [key]: value };
      // Follow the type with its sample, unless the person typed their own URL.
      const samples = Object.values(SAMPLE) as string[];
      const url = !trimmed(v.url) || samples.includes(v.url ?? '') ? (SAMPLE[value as keyof typeof SAMPLE] ?? v.url ?? '') : (v.url ?? '');
      return { ...v, type: value, url };
    },
  },
  {
    id: 'sticker',
    label: 'Figurinha',
    group: 'media',
    description: 'Figurinha a partir de uma imagem (o servidor converte para WebP).',
    recipient: true,
    fields: [{ key: 'sticker', label: 'URL da imagem', control: 'input', inputMode: 'url', hint: 'WebP, PNG ou JPEG públicos.' }],
    defaults: { sticker: SAMPLE.sticker },
    build: (number, v) => ({ number, sticker: trimmed(v.sticker) }),
    validate: (v) => urlError(v.sticker ?? '', 'A URL da imagem'),
  },
  presetKind('button', {
    id: 'button',
    label: 'Botões',
    description: 'Botões de resposta, copiar, abrir link, ligar ou Pix.',
    first: 'btn_reply_1',
    note: {
      tone: 'info',
      text: 'Botões de resposta (até 3) e CTAs agrupados aparecem no celular e no WhatsApp Web. O Pix só aparece no celular.',
    },
  }),
  listKind(),
  presetKind('carousel', {
    id: 'carousel',
    label: 'Carrossel',
    description: 'Cartões deslizantes com imagem, texto e botões.',
    first: 'carousel_reply',
  }),
  {
    id: 'status/text',
    label: 'Status texto',
    group: 'status',
    description: 'Publica um status de texto na conta.',
    recipient: false,
    confirm: 'Entendo que o status será publicado e visto pelos contatos desta conta, não só pelo número de teste.',
    fields: [{ key: 'text', label: 'Texto do status', control: 'textarea', rows: 3 }],
    defaults: { text: '' },
    build: (_n, v) => ({ text: v.text }),
    validate: (v) => required(v, [['text', 'o texto do status']]),
  },
  {
    id: 'status/media',
    label: 'Status mídia',
    group: 'status',
    description: 'Publica um status com imagem ou vídeo na conta.',
    recipient: false,
    confirm: 'Entendo que o status será publicado e visto pelos contatos desta conta, não só pelo número de teste.',
    fields: [
      {
        key: 'type',
        label: 'Tipo',
        control: 'select',
        options: [
          { value: 'image', label: 'Imagem' },
          { value: 'video', label: 'Vídeo' },
        ],
      },
      { key: 'url', label: 'URL do arquivo', control: 'input', inputMode: 'url' },
      { key: 'caption', label: 'Legenda', control: 'input', optional: true },
    ],
    defaults: { type: 'image', url: SAMPLE.image, caption: '' },
    build: (_n, v) => withText({ type: v.type, url: trimmed(v.url) }, { caption: v.caption }),
    validate: (v) => urlError(v.url ?? '', 'A URL do arquivo'),
    patch: (key, value, v) => {
      if (key !== 'type') return { ...v, [key]: value };
      const samples: string[] = [SAMPLE.image, SAMPLE.video];
      const url = !trimmed(v.url) || samples.includes(v.url ?? '') ? (value === 'video' ? SAMPLE.video : SAMPLE.image) : (v.url ?? '');
      return { ...v, type: value, url };
    },
  },
];

export const kindOf = (id: SendKind): KindDef => KINDS.find((k) => k.id === id) ?? (KINDS[0] as KindDef);
export const defaultsOf = (): Record<SendKind, Values> =>
  Object.fromEntries(KINDS.map((k) => [k.id, { ...k.defaults }])) as Record<SendKind, Values>;
