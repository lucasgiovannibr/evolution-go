# 7. Chamadas de voz e vídeo 🧪

> **Recurso experimental.** O servidor consegue **atender, ligar e desligar chamadas** e levar o áudio e o vídeo para o seu sistema, algo que o projeto original não fazia. Foi testado com um número real e um iPhone, mas depende de uma biblioteca de terceiros ainda em desenvolvimento (`purpshell/meowcaller`). Por isso vem **desligado** e só funciona se você ligar, por instância.

## Dois modos

| Modo | Como ligar | O que faz |
|---|---|---|
| **Só rejeitar** (padrão) | nada a fazer | Avisa que alguém ligou (evento `CALL`) e pode rejeitar a ligação, com uma mensagem automática opcional. |
| **Motor de chamadas** | interruptor **Chamadas** na instância | Atende, liga, desliga, controla o vídeo e leva áudio e vídeo por uma conexão especial (WebSocket). |

Com o motor ligado, **toda ligação recebida é pré-aceita** pela biblioteca (o outro celular passa a mostrar a chamada como "em preparação"). Por isso o recurso não vem ligado por padrão.

## Ligar o motor

No painel: instância → **Comportamento** → **Chamadas**. Vale na **próxima conexão** (desconecte e conecte de novo, ou reinicie o servidor).

**Não funciona em instâncias com proxy:** o som e a imagem da chamada saem por uma conexão direta que ignoraria o proxy e mostraria o IP do servidor. O painel e a API avisam (`blocked_by_proxy`).

## Atender e ligar pelo navegador (sem programar)

Na instância, aba **Chamadas**:

- **Chamadas agora**: lista as ligações em andamento. Numa que está tocando: **Atender no navegador** ou **Rejeitar**. Nas demais: **Falar pelo navegador** e **Desligar**.
- **Ligar**: digite o número com DDI e DDD. Marque **Videochamada** para ligar a câmera do computador.
- **Vídeo**: o vídeo do outro lado aparece na página, em pé, e a sua câmera aparece pequena no canto. Se o outro lado pedir para passar a vídeo, o painel **aceita na hora** (pode desligar essa opção).
- **Histórico**: lista de ligações com filtros e botão para apagar tudo. Só guarda metadados (quem, quando, quanto durou, como terminou). Desligado por padrão (`CALL_HISTORY`).

Requisitos:

| Requisito | Por quê |
|---|---|
| Use **fones de ouvido** | Sem eles o alto-falante volta para o microfone e o outro lado se ouve de volta. |
| Página **segura** (`https` ou `localhost`) | O navegador só libera o microfone nesses casos. |
| Chrome, Edge ou Safari recentes | Para o vídeo (WebCodecs e H.264). Sem isso a chamada segue só com áudio. |
| Uma chamada por vez no navegador | As outras podem ser atendidas pela API. |

Se fechar a aba, o áudio é solto e a chamada cai depois de alguns segundos (`CALL_STREAM_GRACE`, 10 s).

## Para quem programa: levar o áudio para um robô

Um sistema seu (por exemplo um agente de IA) pode atender e conversar usando a API:

1. `POST /call/stream-ticket` devolve um bilhete de uso único e o endereço de um WebSocket.
2. Abra o WebSocket **antes** de atender.
3. `POST /call/answer` atende; ou `POST /call/dial` para ligar.
4. Receba e envie áudio (PCM 16 kHz por padrão; também 8/24 kHz, μ-law e A-law) em JSON ou em quadros binários, com `timestamp` em cada quadro.
5. O stream também avisa quando o outro lado começa e termina de falar (`speech_start` / `speech_end`) e confirma quando o áudio enviado foi tocado (`mark`), útil para interromper a fala de um robô.

Tudo em [Chamadas, guia técnico](../wiki/guias-api/api-call.md).

## Limites e cuidados

- **Não há gravação** de chamadas, de propósito (privacidade e legislação, como a LGPD).
- **Sem chamadas em grupo.**
- Há limites configuráveis: chamadas simultâneas (`CALL_MAX_CONCURRENT`, 4), tempo de toque (`CALL_RING_TIMEOUT`, 90 s), discagens por minuto (`CALL_DIAL_LIMIT`, 6) e, se quiser, duração máxima e corte por silêncio.
- Cuidado ao ligar: discar em volume é um dos comportamentos que o WhatsApp mais pune.

## O que foi e o que não foi testado

| ✅ Testado ao vivo (iPhone) | ⬜ Não testado |
|---|---|
| Áudio nos dois sentidos | Android |
| Videochamada recebida e enviada | Chamada em grupo |
| Passar de áudio para vídeo, nos dois sentidos | Chamadas longas (horas) |
| Atender, ligar, silenciar e desligar pelo painel | Câmera real no painel (só a câmera simulada do Edge) |
| Histórico de chamadas | Várias contas, WhatsApp Business e contatos por LID |
