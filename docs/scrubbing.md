# Time Scrubbing Bidirecional

Este guia explica o sistema de time scrubbing do `runtime`: como voltar e
avancar no tempo da simulacao de forma nao destrutiva, e como o determinismo
verificado por checksum torna isso possivel.

## Ideia principal

A engine tem dois mecanismos de viagem no tempo, construidos sobre bases
diferentes:

```text
rewind    (Back/BackToTick)  ->  snapshots     ->  janela limitada, destrutivo
scrubbing (ScrubToTick)      ->  replay do log ->  faixa completa, nao destrutivo
```

O **rewind** restaura snapshots completos do estado (World, input, UI, estado
do jogo) capturados a cada tick. A janela e limitada por `Config.HistoryLen`
e cada volta consome o historico — e um undo.

O **scrubbing** nao guarda o futuro: ele **reconstroi** qualquer tick
re-simulando a partir do tick 0 com o log completo de input. Como a simulacao
e deterministica, o mesmo log produz exatamente o mesmo estado, sempre. Por
isso arrastar o slider de `100 -> 50 -> 100` cai bit a bit no mesmo tick 100
de onde saiu.

## Como funciona

O `Driver` mantem um log always-on de todos os eventos de input, carimbados
com o tick em que foram aplicados. `ScrubToTick(alvo)`:

1. re-executa o jogo num driver interno, do tick 0 ate o alvo, injetando os
   eventos do log nos ticks exatos em que aconteceram (via `runtime.Replay`);
2. adota o estado reconstruido: World, input, UI, estado do jogo e a janela
   de snapshots do rewind, todos consistentes com o tick alvo.

A faixa alcancavel e `[0, MaxRecordedTick]` (exposta em
`DriveState.MaxRecordedTick`), independente de `HistoryLen`. Scrub para tras
nunca abaixa esse teto — o teto so muda quando a timeline forka (abaixo).

```text
ControlScrubToTick{Tick: alvo}  ->  pausa + re-simula 0..alvo  ->  estado exato
```

O web canvas liga o slider da timeline a esse comando; qualquer plataforma
pode fazer o mesmo via `Driver.Control`.

## Fork da timeline

Rebobinar e *assistir* nao muda nada. A timeline so diverge no momento em que
um **step de simulacao roda a partir de um tick anterior ao teto** — a partir
dali o que acontece ao vivo nao e mais o futuro gravado (os eventos gravados
nao sao reinjetados no play ao vivo). Nesse instante o Driver forka:

- o futuro gravado apos o ponto do fork e descartado;
- o teto (`MaxRecordedTick`) desce para o ponto do fork e volta a crescer
  com o play;
- scrub apos o fork reconstroi a **nova** ramificacao — a que o usuario
  assistiu — e o passado anterior ao fork continua intacto.

Input sozinho nunca forka. Eventos que chegam com a simulacao rebobinada e
pausada (mover o mouse sobre o canvas, por exemplo) ficam num staging: sao
commitados ao log se um step consumir o estado deles (fork), ou descartados
se outro scrub/back/reset rebobinar antes. Isso garante que "so olhar" nunca
destroi o futuro.

## Checksum: a prova de que o scrub e exato

`World.Checksum()` e um hash deterministico do estado da simulacao, e
`Driver.Checksum()` o expoe. Dois estados com o mesmo checksum sao o mesmo
estado — e assim que os testes provam que o scrubbing e exato:

```text
run original:  checksum a cada tick
scrub 15, 45, 5, 60, 30:  checksum de cada salto == checksum original do tick
```

O mesmo mecanismo verifica gravacoes explicitas:

```go
driver.StartRecording()            // no tick 0
// ... roda o jogo ...
rec := driver.StopRecording()

replay := runtime.Replay(&MeuJogo{}, log, rec)
replay.Checksum() == driver.Checksum() // reproducao bit a bit
```

`Replay` reseta e re-executa a instancia de jogo recebida — passe uma
instancia nova, nao a que estiver acoplada a um driver vivo. Gravacoes devem
comecar no tick 0 (driver novo ou logo apos reset); um reset no meio de uma
gravacao reinicia a captura, porque os ticks recomecam do zero.

## O contrato: seu jogo precisa ser deterministico

O scrubbing herda os requisitos do replay. Dentro de `Update`:

- nada de RNG global (`math/rand` sem seed) — derive aleatoriedade de
  `ctx.Tick`;
- nada de tempo de parede, I/O ou estado fora do World/`StateSlot`;
- `StateSlot[T]` com campos slice/map/pointer precisa implementar
  `Clone() T` (veja `runtime.Cloneable`).

Se o jogo quebrar o contrato, o scrub reconstroi um estado diferente do que
foi assistido — o checksum por tick no log de debug do Driver existe para
denunciar exatamente isso.

## Custos e limites

- **Custo por scrub**: O(tick alvo) — cada salto re-simula desde o tick 0.
  Para sessoes de dev e demos isso e barato; keyframes esparsos sao a
  evolucao natural se sessoes longas apertarem.
- **Memoria**: o log guarda apenas eventos de input (pequenos), nunca
  snapshots do mundo. `ControlReset` zera log e teto.
- **Rewind continua existindo**: `Back`/`BackToTick` seguem uteis como undo
  rapido de curto alcance sem custo de re-simulacao; o scrubbing cobre a
  faixa completa.
