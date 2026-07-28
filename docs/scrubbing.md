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

## Modo assistir

Rebobinar e reconstruir um tick e so metade da jornada: ate aqui, o unico
jeito de seguir e rodar a simulacao ao vivo, o que forka a partir
dali. O modo assistir e o segundo jeito: reproduzir a gravacao tick a tick --
play, pause, step, velocidade -- sem nunca forkar.

```text
ramificar (play ao vivo a partir de um tick rebobinado)  -> fork
assistir  (ControlReplayPlay/Pause/Step)                 -> consome o journal, nunca forka
```

Enquanto assistindo, o `Driver` avanca lendo eventos e mudancas de viewport ja
gravados, na mesma ordem canonica de um step normal. Nao ha escrita: nenhum
fork, nenhuma truncagem do log, nenhum commit de input staged. Input ao vivo
que chegar durante o modo fica em staging (a mesma politica do scrub) e e
descartado a cada avanco -- "so assistir" nao destroi o futuro, exatamente
como "so olhar" via scrub.

### Comandos

| Comando | Efeito | Sai do modo? |
|---|---|---|
| `ControlReplayPlay` | entra no modo (se fora) e toca a partir do tick atual | nao |
| `ControlReplayPause` | pausa o playback | nao (permanece) |
| `ControlReplayStep` | entra no modo (se fora), avanca 1 tick e pausa | nao |
| `ControlPlay` | assume controle: retoma play ao vivo | **sim** |
| `ControlStep` | assume controle: avanca 1 tick ao vivo | **sim** |
| `ControlPause` | pausa; equivalente a `ControlReplayPause` dentro do modo | nao |
| `ControlScrubToTick` / `ControlBack*` | reposiciona, pausa | nao |
| `ControlReset` | reseta tudo | sim |

`ControlPlay`/`ControlStep` sao o gesto explicito de "assumir controle": o
proximo step forka a partir dali, exatamente como hoje. Reposicionar (scrub,
back) nunca e esse gesto -- so pausa e permanece assistindo.

Ao alcancar o tick final gravado (`MaxRecordedTick`), o playback pausa
automaticamente e permanece no modo -- o usuario pode rebobinar e reassistir,
ou assumir controle dali (o que e play normal, sem perda, ja que o tick final
ja e o topo da timeline). Velocidade (`ControlSetSpeed`) e o catch-up clamp do
relogio se aplicam ao playback do mesmo jeito que ao play ao vivo: o modo
assistir so troca a fonte dos ticks (journal em vez de `Game.Update` ao vivo),
nao o agendamento.

Cues de audio sao audiveis durante o playback -- e a primeira audicao de quem
carregou a gravacao. Durante o scrub elas continuam mudas, porque ja foram
ouvidas na execucao original. Veja `docs/audio.md`.

### `ReplayForViewing`

`runtime.Replay` reconstroi so o estado final, sem manter keyframes -- e o
suficiente para verificacao e testes. `runtime.ReplayForViewing` reconstroi
igual, mas mantem o recorder de keyframes ligado durante a reconstrucao e
devolve o driver pausado, no modo assistir, no tick final. E a funcao certa
para adotar uma gravacao num host interativo: scrub para tras fica barato
desde o primeiro salto (nao precisa esperar a janela densa se formar de novo)
e o driver ja sai pronto para tocar.

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

- nada de RNG global (`math/rand` sem seed) — use `ctx.Rand`
  (`UpdateContext.Rand`);
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
