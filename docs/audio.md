# Audio

Este guia explica o modelo de audio do FrameForge: jogos declaram clips,
emitem eventos deterministas em `ctx.Audio`, e hosts consomem a saida pelo
`runtime.Driver`.

## Modelo

Audio tem duas formas:

```text
cue    -> som pontual, fire-and-forget, carimbado com tick e seq
track  -> som continuo desejado, reconciliado pelo host
```

Use cues para efeitos como impacto, confirmacao de UI ou erro. Use tracks
para musica, ambiente ou loops longos. O runtime nao decodifica nem toca som;
ele apenas transporta intencoes deterministicas ate o host.

## API do jogo

`runtime.UpdateContext` expoe `ctx.Audio` em `Init`, `Update` e `PostStep`.
Clips sao declarados no mesmo `manifest.json` usado por texturas e sprites.

```json
{
  "version": 1,
  "audio": [
    { "id": "hit", "file": "hit.wav" },
    { "id": "music", "file": "music.wav" }
  ]
}
```

```go
func (g *Game) Init(ctx *runtime.UpdateContext) {
	ctx.Audio.SetTrack("music", audio.Track{ClipID: "music", Loop: true, Gain: 0.4})
}

func (g *Game) PostStep(ctx *runtime.UpdateContext, stats scene.StepStats) {
	ctx.Audio.PlayWith("hit", audio.CueOptions{Gain: 0.8})
}
```

`Gain` e linear. O valor zero significa 1.0, para manter structs zero-value
uteis. `Pan` fica em `[-1, 1]`.

## Contrato de host

Hosts chamam `Driver.ConsumeAudio()` depois de avancar a simulacao, no mesmo
contexto serializado que chama `AdvanceTime` ou `Step`.

```text
ApplyInput -> AdvanceTime/Step -> ConsumeAudio -> entregar ao backend de audio
```

`ConsumeAudio` drena cues pendentes e retorna uma copia das tracks desejadas.
Cues acumulam ate o consumo, com fila limitada. Tracks sao estado continuo:
o host compara o desejado com o que esta tocando, inicia o que falta, para o
que sobrou e ajusta ganho quando necessario.

O web canvas serve em `/audio/<path>` os clips declarados em
`Config.Assets.AudioClips()` quando recebe um `AudioFS`, envia um manifest
para novos clientes e transmite mensagens de audio pelo WebSocket junto ao
loop de simulacao.

## Rewind, scrub e replay

Audio segue a timeline da simulacao, mas nao entra em `World.Checksum()`.

| Operacao | Cues | Tracks |
|---|---|---|
| Play normal | emitidos e drenados uma vez | reconciliadas pelo host |
| Rewind | cues pendentes do futuro sao descartados | restauradas pelo snapshot |
| Scrub | cues re-simulados sao suprimidos | reconstruidas para o tick alvo |
| Fork apos scrub | cues voltam a emitir na nova timeline | seguem o novo estado |
| Replay explicito | nao altera checksum | estado e reconstruido |

Emita audio apenas em `Init`, `Update` ou `PostStep`. Nunca em `Render`: frames
podem rodar em frequencia diferente dos ticks e quebrariam determinismo.
