# Boot Splash

## Fase de runtime

`runtime.Driver.RunSplashFrame` possui a fase anterior ao tick 0.

- Hosts WASM e native em tempo real fornecem viewport e input durante a splash.
- Raster e replay pulam a splash e começam no tick 0.
- O tempo da splash usa apenas o relógio de parede.
- O tempo da splash não avança tick, recorder, checksum ou estado de rewind.
- Um segmento skippable só avança depois de input de ativação.

## Playlist

A playlist pode conter um segmento opcional da engine seguido por segmentos
`"splash"` do manifest.

O segmento da engine é uma animação compilada de `internal/enginesplash`. Ele é
renderizado como comandos vetoriais sem textura. Não possui entrada no manifest
nem exige upload de textura. `Config.DisableDefaultSplash` o desativa.

Segmentos do jogo são flip-books de textura vindos do manifest. Seus campos são
`cell_w`, `cell_h`, `columns`, `frames`, `rate`, `min_time`, `fade` e
`skippable`. Use `arasy atlas <frames-dir|gif> -o out.png` para empacotar frames.

## Opções de host e web

- `--no-engine-splash` é uma opção apenas do host. Ela mantém segmentos do
  manifest e fica ativa por padrão em `arasy dev`.
- `--no-loading` controla somente o indicador de download HTML.
- `--no-splash` controla somente os segmentos de splash de runtime.
- `--boot-background` define a cor da fase web 0.
- `Config.BootBackground` define a cor da splash de runtime e cai para
  `Config.Clear`.

O comportamento de tela de loading `HoldUntilReady` não está implementado. Cada
  segmento é limitado atualmente por `MinTime`.
