# WhatyGo Passkey Helper

Extensão de navegador que conclui o pareamento por **chave de acesso (passkey)**
do WhatsApp Web para o WhatyGo.

Desde que o WhatsApp passou a exigir passkey ao vincular um novo dispositivo, o
QR Code sozinho não basta em algumas contas: é preciso executar uma cerimônia
WebAuthn (`navigator.credentials.get`), e o navegador só permite isso quando o
código roda no origin da _relying party_ — `whatsapp.com`. Esta extensão roda
**somente** em `web.whatsapp.com` e serve apenas como "palco" com o origin
correto para a biometria/PIN. Ela **não** interage com o login do WhatsApp Web e
**não** coleta nem envia dados para terceiros.

## Como funciona

1. Você escaneia o QR normalmente no WhatyGo (manager). Se a conta exigir
   passkey, o painel de conexão mostra o botão **"Abrir WhatsApp Web"**.
2. Esse botão abre `https://web.whatsapp.com/#wapk=<payload>`, onde `payload` é
   um base64url de `{ "t": "<token da instância>", "b": "<url base da API>" }`.
3. O content script lê esse payload, consulta o status da cerimônia na API,
   executa a cerimônia WebAuthn no origin `whatsapp.com` e envia a assinatura de
   volta para o WhatyGo.
4. Se o pareamento exigir confirmação manual do código, o painel mostra o código
   e um botão **"Confirmar código"**. Na maioria dos casos (QR escaneado antes),
   o WhatyGo confirma automaticamente e você só precisa voltar ao manager.

Nenhuma `host_permission` é necessária: as chamadas partem do origin
`web.whatsapp.com` e o backend do WhatyGo já libera CORS para essa origem.

### Gerenciadores de senha (1Password, Bitwarden…) — v1.1.0

A chamada `navigator.credentials.get()` roda em um script separado
(`webauthn-main.js`) declarado com `"world": "MAIN"`, isto é, no mesmo mundo
JavaScript da página. Gerenciadores de senha só interceptam o WebAuthn ali; a
partir do content script (mundo isolado) o Chrome pedia "Insira sua chave de
segurança". O script do mundo MAIN **não faz requisições de rede** (o CSP do
WhatsApp Web as bloquearia): recebe o desafio do `content.js` por
`window.postMessage` e devolve a assinatura. Também não força `transports` nem
envia `allowCredentials` vazio, para o navegador escolher o autenticador.
Requer **Chrome/Edge 111 ou superior**.

## Endpoints usados (WhatyGo)

- `GET  {base}/passkey-ceremony/{token}` — status/desafio da cerimônia
- `POST {base}/passkey-ceremony/{token}/response` — envia a assertion WebAuthn
- `POST {base}/passkey-ceremony/{token}/confirm` — confirma o código (quando aplicável)

Onde `base` é a URL da sua API WhatyGo e `token` é o token da instância.

## Instalar no Google Chrome

1. Baixe/descompacte esta pasta (a que contém `manifest.json`).
2. Abra `chrome://extensions`.
3. Ative o **Modo do desenvolvedor** (canto superior direito).
4. Clique em **Carregar sem compactação**.
5. Selecione esta pasta.
6. Volte ao WhatyGo e clique em **"Abrir WhatsApp Web"**.

## Instalar no Microsoft Edge

1. Baixe/descompacte esta pasta (a que contém `manifest.json`).
2. Abra `edge://extensions`.
3. Ative o **Modo de desenvolvedor** (canto inferior esquerdo).
4. Clique em **Carregar sem pacote**.
5. Selecione esta pasta.
6. Volte ao WhatyGo e clique em **"Abrir WhatsApp Web"**.

## Dicas

- Use o navegador logado na mesma conta (Google/iCloud) onde a passkey do
  WhatsApp está salva, ou tenha o celular por perto para o fluxo de biometria.
- A cerimônia tem validade curta. Se expirar, gere um novo QR no WhatyGo.

## Whitelabel

A extensão é genérica: nada de URL ou token fica embutido no código — tudo vem
no `payload` da URL. Para personalizar nome/ícone, edite o `manifest.json` e o
título/descrição do painel em `content.js`.
