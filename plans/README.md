# Planos e documentos — índice

Todos os documentos de planejamento do frameforge vivem em `plans/`, organizados
em duas subpastas por natureza do documento:

- **`plans/direcao/`** — documentos de direção e review: revisões técnicas,
  roadmaps, decisões arquiteturais (ADRs), visão estratégica, design e esboços.
  Não têm entregável de código próprio; guiam e contextualizam o trabalho.
- **`plans/implementacao/`** — planos de implementação executáveis (com escopo,
  passos, critérios de aceite e condições de STOP). Planos concluídos têm **✓**
  no nome do arquivo (ex.: `008?-runtime-pump-decomposicao-driver.md`).
- **`plans/assets/`** — concept arts referenciados pelos planos.
- **`plans/README.md`** — este índice.

A numeração é **cronológica** (ordem de criação), não de prioridade — um
documento antigo e já implementado não perde seu número. A ordem de **execução
recomendada** para o trabalho pendente está na seção própria abaixo, separada da
numeração.

**Nota sobre git**: `plans/` está no `.gitignore` — estes documentos são notas
locais do mantenedor, não versionadas.

**Reorganização de 2026-07-07**: planos separados em `direcao/` e
`implementacao/`; planos de implementação concluídos marcados com ? no nome do
arquivo. Substitui a reorganização anterior de 2026-07-03. Cada documento foi
verificado contra o código real em `HEAD` (commit `4ec52df`) antes de receber seu
status — não confie apenas no que o próprio documento diz sobre si mesmo.

## Direção e Review

| # | Título | Tipo | Data | Status | Depende de |
|---|--------|------|------|--------|------------|
| [001](direcao/001-revisao-arquitetural.md) | Revisão Arquitetural e de Design | Review | 2026-06-26 | REFERÊNCIA — achados tratados nas rodadas seguintes (revisão técnica 009, ADRs 010) | — |
| [002](direcao/002-api-publica-e-logs.md) | Roadmap: API Pública e Estrutura de Logs | Roadmap | 2026-06-27 | DONE — layering, `Game`/`Context`/`Driver`/logging shipped; só `platform/native` (item "futuro" do próprio doc) segue TODO | — |
| [003](direcao/003-imgui.md) | Roadmap: IMGUI na frameforge | Roadmap | 2026-06-28 | PARCIAL — fases 1–5 DONE (pointer, `ui/`, integração runtime, demo football); fase 6 (control commands no input log) TODO; fase 7 extraída para [013](direcao/013-animacao-ui.md) | — |
| [004](direcao/004-melhorias-api-publica-runtime.md) | Melhorias de design — API pública / `runtime` | Design | 2026-06-29 | REFERÊNCIA — maior parte absorvida por [010](direcao/010-decisoes-arquiteturais.md) e [003](direcao/003-imgui.md); split `UpdateContext`/`RenderContext` já implementado | — |
| [006](direcao/006-animacao-mundo.md) | Roadmap: animação de componentes do mundo | Roadmap | 2026-06-29 | SUBSTITUÍDO — reescrito em [012](direcao/012-animacao-simulacao.md); fica como histórico | — |
| [007](direcao/007-renderizacao.md) | Roadmap de renderização — assets, transforms, GPU e efeitos | Roadmap | 2026-06-29 | TODO — Tier 0 (assets/`DrawImage`) não iniciado apesar do doc dizer "esboço"; Tiers 1–3 são direção, não plano travado | — |
| [009](direcao/009-revisao-tecnica.md) | Revisão Técnica do frameforge | Review | 2026-07-01 | REFERÊNCIA — maioria dos achados originais já corrigida (ver tabela de status no próprio doc); itens em aberto viraram os planos 010–023 | — |
| [010](direcao/010-decisoes-arquiteturais.md) | Decisões Arquiteturais (AD-1…AD-9) | ADR | 2026-07-01 | REFERÊNCIA — registro vivo. AD-1 **DONE** ([015✓](implementacao/015✓-auto-step-ad1.md), 2026-07-03), AD-2 **DONE** núcleo runtime (`StartRecording`/`StopRecording`/`Replay` + testes), AD-3 **DONE** (`debug/` removido, `9d6298c`), AD-4 REFERÊNCIA (direção), AD-5 **DONE** (`basicfont` no raster), AD-6 **DONE** (`render.RGBA8`, `9d6298c`), AD-7 **DONE** ([017✓](implementacao/017✓-bounds-restituicao-ad7.md), 2026-07-03), AD-8 WON'T DO, AD-9 **DONE** ([016✓](implementacao/016✓-checksum-completo-ad9.md), `a5b5df7` + testes 2026-07-03) | — |
| [011](direcao/011-visao-diferenciais.md) | Visão de Diferenciais | Visão | 2026-07-01 | REFERÊNCIA — discussão estratégica, sem entregável próprio | — |
| [012](direcao/012-animacao-simulacao.md) | Roadmap: animação de simulação (determinística) | Roadmap | 2026-07-01 | **DONE** — `ease/`, `TransformTween`, `CircleSpriteTween`, `AdvanceAnimations`, checksum/clone/despawn e APIs `TweenTransform`/`TweenCircleSprite`; football usa tween determinístico no reset/blink da bola | — |
| [013](direcao/013-animacao-ui.md) | Roadmap: animação de UI (render-only) | Roadmap | 2026-07-01 | **DONE** — `Frame(elapsed)`, `AnimStore`, primitivas (`Approach`/`Spring`/`Tween`/`SequenceKeyed`), botões animados, `UI.Effect`, `UI.FX`/`Toast`, docs e banner de gol | — |
| [033](direcao/033-tooling-debug-inspetor.md) | Tooling de debug — debugger como query sobre a timeline | Direção | 2026-07-04 | REFERÊNCIA — desdobramento da Aposta 3 de [011](direcao/011-visao-diferenciais.md); cada ferramenta vira plano quando atacada | AD-2/AD-9 (satisfeitas); bisect e timeline melhoram após [023](implementacao/023-recorder-keyframes-persistencia.md) |
| [034](direcao/034-diferenciais-voltados-ao-jogador.md) | Diferenciais voltados ao jogador | Direção | 2026-07-04 | REFERÊNCIA — tradução dos diferenciais de [011](direcao/011-visao-diferenciais.md) para features de jogador; vira plano só quando o jogo de manager existir | [023](implementacao/023-recorder-keyframes-persistencia.md) é pré-requisito prático; gameplay depende de [020✓](implementacao/020✓-ui-manager-fundacoes.md)/[021✓](implementacao/021✓-demo-manager.md)/[022](implementacao/022-football-expandido.md) |
| [035](direcao/035-motor-de-partidas.md) | Esboço do motor de simulação de partidas (estado da arte) | Esboço | 2026-07-04 | REFERÊNCIA — design do coração do **jogo comercial privado** (repo próprio, consome frameforge como lib); fases G0–G7; **correções de 2026-07-06** (passeio de quadrantes): 2.5D?engine, portão de criação do repo, web-only, CI x86-64/arm64/wasm, ~20 ligas, moral/torcida no contexto, justiça percebida | frameforge como biblioteca; [023](implementacao/023-recorder-keyframes-persistencia.md); [022](implementacao/022-football-expandido.md) apenas prova padrões |
| [036](direcao/036-25d-engine-e-ci-determinismo.md) | 2.5D na engine + CI de determinismo cross-arch | Esboço | 2026-07-06 | TODO — último item do portão de criação do repo do jogo; escopo travado pelo passeio de quadrantes (correções no 035); questões abertas: CCD×tick-rate e pacote (decidir com benchmark) | trilha runtime (D10+`Derive` → [023](implementacao/023-recorder-keyframes-persistencia.md) → [022](implementacao/022-football-expandido.md)) e trilha plataforma ([028✓](implementacao/028✓-wasm-host.md) → [029✓](implementacao/029✓-webgpu-renderer.md)) |
| [040](direcao/040-roadmap-visual-match-render.md) | Roadmap visual do match render — do Tier 0 ao concept "Brasfoot NEXT" (fases V0–V4) | Esboço | 2026-07-06 | TODO — especializa o [007](direcao/007-renderizacao.md) para os concepts de `plans/assets/`; identifica 2 planos faltantes (texturas dinâmicas, Tier 1 transform) + fontes (007 fase C); Tier 3 fica atrás de decisão explícita (AD novo) | V0 = [037-assets](implementacao/037-asset-core-drawimage.md)→[038](implementacao/038-raster-drawimage.md)→[039](implementacao/039-canvas-drawimage.md); V2 = [036](direcao/036-25d-engine-e-ci-determinismo.md); V3 = [028✓](implementacao/028✓-wasm-host.md)→[029✓](implementacao/029✓-webgpu-renderer.md) |

## Implementação

Planos concluídos têm **✓** no nome do arquivo e no link abaixo.

| # | Título | Data | Status | Depende de |
|---|--------|------|--------|------------|
| [005](implementacao/005✓-demo-breakout.md) | Demo Breakout | 2026-06-29 | PARCIAL/DIVERGENTE — demo existe e roda (`cmd/demos/breakout/`), mas foi reconstruída sobre `scene.RectCollider` (engine) em vez de colisão local, ao contrário do plano (§1.1) | — |
| [008✓](implementacao/008✓-runtime-pump-decomposicao-driver.md) | Driver dono do tempo (pump) + decomposição do Driver | 2026-07-01 | **DONE** — `Driver.AdvanceTime`/`Step` e decomposição em colaboradores (`f357632`, `8c908b9`, `e0df2eb`) | — |
| [014✓](implementacao/014✓-colisao-circle-rect.md) | Colisão Circle×Rect e Rect×Rect | 2026-07-02 | **DONE** — `RectCollider`, narrow phase circle×rect/rect×rect, broad phase misto, breakout migrado, checksum, bounds/bounce e overlay de debug para rects entregues | — |
| [015✓](implementacao/015✓-auto-step-ad1.md) | Auto-Step com HoldSimulation e PostStep (AD-1) | 2026-07-02 | **DONE** (2026-07-03) — Driver faz auto-step, `HoldSimulation` segura por tick, `PostStepper` recebe contatos do mesmo tick; demos migrados | [008✓](implementacao/008✓-runtime-pump-decomposicao-driver.md) (satisfeita) |
| [016✓](implementacao/016✓-checksum-completo-ad9.md) | Checksum de estado completo (AD-9) | 2026-07-02 | **DONE** (`a5b5df7` + testes 2026-07-03) — 9 mapas de componente + `nextID`/`stepTick` cobertos, sensibilidade por componente e ausente?zero testados, guarda estrutural por reflexão | — |
| [017✓](implementacao/017✓-bounds-restituicao-ad7.md) | Bounds por lado + restituição nas paredes (AD-7) | 2026-07-02 | **DONE** (2026-07-03) — `BounceConfig` com `OpenSides`/`UseBodyRestitution`, runtime plumbing e breakout sem hack de bounds | [014✓](implementacao/014✓-colisao-circle-rect.md) (satisfeita) |
| [018✓](implementacao/018✓-ci-github-actions.md) | CI com GitHub Actions | 2026-07-02 | **DONE (essencialmente)** — `.github/workflows/ci.yml` + badge + limpeza AD-3/AD-6 (`9d6298c`); falta só documentar os gates no AGENTS.md | — |
| [019✓](implementacao/019✓-golden-frame-tests.md) | Golden frame tests | 2026-07-02 | **DONE** (2026-07-04) — goldens PNG pixel-exatos em `platform/raster`, golden JSON em `internal/render/adapter/web`, fluxo `-update`, arquivos de falha e docs de regeneração | — |
| [020✓](implementacao/020✓-ui-manager-fundacoes.md) | Fundações de UI para telas de manager (parte 1/2) | 2026-07-02 | **DONE** — `Clip`, `CornerRadius`, `DrawPolygon`, rotação/medição de texto, wheel, foco, scroll, listas, tabelas, cards, badges e temas entregues em `render/`, `input/` e `ui/` | — |
| [021✓](implementacao/021✓-demo-manager.md) | Demo "Manager" — telas de gestão sem física (parte 2/2) | 2026-07-02 | **DONE** — implementado como `cmd/demos/uimanager/`: dashboard, tabela, elenco, calendário, tema Matchday, navegação, scroll/seleção e host canvas | [020✓](implementacao/020✓-ui-manager-fundacoes.md) |
| [022](implementacao/022-football-expandido.md) | Football Expandido | 2026-07-02 | TODO — football segue 4v4 sem IA/zonas; D10/RNG global foi extraído e concluído no [043✓](implementacao/043✓-runtime-rand-derive.md). **Adendo 2026-07-06**: execução adiada para depois da V0 (037-assets→038→039); ganhou fase 9 (visual, delegada ao [042](implementacao/042-sombra-e-altura-football.md)) | [014✓](implementacao/014✓-colisao-circle-rect.md) (satisfeita); [043✓](implementacao/043✓-runtime-rand-derive.md) (satisfeita); V0 ([037-assets](implementacao/037-asset-core-drawimage.md)→[038](implementacao/038-raster-drawimage.md)→[039](implementacao/039-canvas-drawimage.md)) |
| [023](implementacao/023-recorder-keyframes-persistencia.md) | Recorder por keyframes, seek unificado e persistência | 2026-07-02 | PARCIAL — Fases 1–2 DONE, Fase 3 aguardando D5 | — |
| [024✓](implementacao/024✓-fix-bounce-rect-precedence.md) | Precedência D1 no BounceEntitiesInside + testes de bounce com rect | 2026-07-02 | **DONE** (2026-07-03) | — |
| [025✓](implementacao/025✓-mixed-collider-tests-benchmarks.md) | Testes e benchmarks do sistema misto de colisores | 2026-07-02 | **DONE** (2026-07-03) | [024✓](implementacao/024✓-fix-bounce-rect-precedence.md) |
| [026✓](implementacao/026✓-docs-drift-and-gomod.md) | Drift de docs (AGENTS.md/README) + go.mod | 2026-07-02 | **DONE** — paths de planos já corrigidos, `golang.org/x/image` direto em `go.mod`, e drift restante de raster/README corrigido em 2026-07-07 | — |
| [027](implementacao/027✓-collision-api-surface-cleanup.md) | Deprecação da API pública de colisão + status de `loop.FixedStep` | 2026-07-02 | SUBSTITUÍDO — absorvido por [030✓](implementacao/030✓-public-api-surface-cleanup.md) com remoção direta (decisão do mantenedor, 2026-07-04) | — |
| [028✓](implementacao/028✓-wasm-host.md) | Host WebAssembly (rAF, input DOM, renderer Canvas2D provisório) | 2026-07-02 | **DONE** — `platform/wasm`, input DOM, rAF loop, Canvas2D provisório, demo breakout WASM, shell HTML/`wasm_exec.js` e `cmd/wasmserve` entregues | — |
| [029✓](implementacao/029✓-webgpu-renderer.md) | Renderer WebGPU para o host WASM | 2026-07-02 | **DONE** (2026-07-07) — tesselação CPU em `platform/wasm/tess`, shaders WGSL embutidos, WebGPU default, fallback Canvas2D via `ForceCanvas2D`, demo breakout validado no browser; fonte bitmap inicial fica para o plano 041 | [028✓](implementacao/028✓-wasm-host.md) |
| [030✓](implementacao/030✓-public-api-surface-cleanup.md) | Enxugar a superfície pública do engine + redesenho de `ToastOptions` | 2026-07-04 | **DONE** (2026-07-04) — pipeline/debug/colisão de `scene` privatizados, aliases `Stop*Tween` removidos, acessores redundantes do Driver removidos, `loop.FixedStep` deletado, `input.Is*Code` privatizado, `ToastOptions` reduzido | — |
| [031✓](implementacao/031✓-transicoes-de-tela.md) | Transições de tela na UI — grupos de comandos + `ScreenTransition` keyed | 2026-07-04 | **DONE** (2026-07-05) — render-only, aditivo em `ui/`; `GroupOptions`, `ScreenTransition`, receitas Fade/Slide, docs, testes e consumo no breakout | [030✓](implementacao/030✓-public-api-surface-cleanup.md) (satisfeita) |
| [032✓](implementacao/032✓-ecs-sparse-set-storage.md) | Storage do `scene.World`: maps → sparse sets densos ordenados | 2026-07-04 | **DONE** (2026-07-08) — stores densos ordenados, checksum preservado, `Clone` lazy sem copiar `sparse`, `entityCount` cacheado; comparação inicial @1000: Clone -79.5%/Step -25.7% spread, Clone -72.8%/Step -25.2% mixed; churn 1M/live1k Clone ~35µs e ~152KB/op | [023](implementacao/023-recorder-keyframes-persistencia.md) (obrigatória) |
| [037-audio](implementacao/037-audio.md) | Suporte a áudio — vocabulário `audio/` agnóstico, `ctx.Audio`, `Driver.ConsumeAudio`, backend Web Audio no canvas | 2026-07-06 | TODO | nenhuma obrigatória; backend WASM segue [028✓](implementacao/028✓-wasm-host.md); [023](implementacao/023-recorder-keyframes-persistencia.md) depois deve incluir `audio.State` |
| [037-assets](implementacao/037-asset-core-drawimage.md) | Núcleo de assets — `render.DrawImage`, pacote `asset/` data-driven, plumbing no runtime | 2026-07-06 | **DONE** (2026-07-08) — `DrawImage`/`TextureID`, `asset.Load` em camadas, `asset/pixel`, `runtime.Assets` e docs/tests aterrissados; aguarda 038/039 para backend de desenho | — |
| [038](implementacao/038-raster-drawimage.md) | `DrawImage` no backend raster + demo de validação offline | 2026-07-06 | TODO — semântica canônica (nearest-neighbor, tint premultiplicado, rotação no centro, placeholder magenta) que canvas/WASM/WebGPU devem espelhar | [037-assets](implementacao/037-asset-core-drawimage.md) (obrigatória) |
| [039](implementacao/039-canvas-drawimage.md) | `DrawImage` no web canvas — DTO do adapter, endpoints `/assets/*` e registry `ImageBitmap` no frontend | 2026-07-06 | TODO — fecha o Tier 0 fase B de [007](direcao/007-renderizacao.md); bytes trafegam 1× por HTTP, frames WS carregam só ID+rects; validação manual no browser | [037-assets](implementacao/037-asset-core-drawimage.md) e [038](implementacao/038-raster-drawimage.md) (obrigatórias) |
| [041](implementacao/041-tipografia-fontes.md) | Tipografia — fontes TTF/OTF data-driven (`fonts` no manifest), `FontID`, métricas determinísticas (`asset/typeface`), raster nítido + `FontFace` no canvas | 2026-07-06 | TODO — fase C do Tier 0 de [007](direcao/007-renderizacao.md); fecha a lacuna de fontes apontada pelo [040](direcao/040-roadmap-visual-match-render.md); `Font` 0 = fonte embutida (goldens intactos); métricas nunca vêm da plataforma | [037-assets](implementacao/037-asset-core-drawimage.md), [038](implementacao/038-raster-drawimage.md) e [039](implementacao/039-canvas-drawimage.md) (obrigatórias) |
| [042](implementacao/042-sombra-e-altura-football.md) | Render 2.5D mínimo na engine — `render.DrawEllipse` + `render.Projection` (sombra elíptica, lift/escala por altura; subconjunto de apresentação do [036](direcao/036-25d-engine-e-ci-determinismo.md) §3, linha do concept no [040](direcao/040-roadmap-visual-match-render.md) §1 que estava sem plano) + integração na demo de football (fase 9 do 022) | 2026-07-06 | TODO — Parte A (engine) executável já; Parte B (demo) bloqueada por [022](implementacao/022-football-expandido.md) fases 1–4 | Parte A: nenhuma; Parte B: [022](implementacao/022-football-expandido.md) |
| [043✓](implementacao/043✓-runtime-rand-derive.md) | `runtime.Rand` + `Derive` (D10) — RNG determinística: splitmix64 tipo-valor, `Config.Seed`, snapshot/scrub/replay, `UpdateContext.Rand`, morte do `rand.Float64()` da demo, adendo AD-10 | 2026-07-06 | DONE — entregue em 2026-07-07; pré-requisito do [023](implementacao/023-recorder-keyframes-persistencia.md) satisfeito | nenhuma |
| [044](implementacao/044-texturas-dinamicas.md) | Texturas dinâmicas (040 §3.A) — `dynamic` no manifest, `RenderContext.UpdateTexture` + `Driver.ConsumeTextureUpdates` (render-only), consumo raster + canvas com versão/re-fetch e invalidação do `tintCache` | 2026-07-06 | TODO — bloqueado pela V0 | [037-assets](implementacao/037-asset-core-drawimage.md)→[038](implementacao/038-raster-drawimage.md)→[039](implementacao/039-canvas-drawimage.md) (obrigatórias) |
| [045](implementacao/045-transform-afim.md) | Tier 1 — transform afim por comando (040 §3.B / 007 §3): `math2d.Affine` + `BaseCommand.Transform` (zero = identidade), geometria transformada no raster/tess, `ctx.transform` nos canvas; hit-testing deferido | 2026-07-06 | TODO — primitivas executáveis já; `DrawImage` transformado exige [038](implementacao/038-raster-drawimage.md) | nenhuma obrigatória (imagens: [038](implementacao/038-raster-drawimage.md)) |
| [046](implementacao/046-webgpu-texturas-quad-projetivo.md) | Texturas no WebGPU (040 §3.C caminho B) — `DrawImage` no backend WebGPU, mipmaps por render pass, sampler anisotrópico, `math2d.Homography` + `render.DrawImageQuad` (quad projetivo; raster segue oráculo, canvas aproxima por grade) | 2026-07-07 | TODO — bloqueado por V0 + [029✓](implementacao/029✓-webgpu-renderer.md); marco V3.5 do [040](direcao/040-roadmap-visual-match-render.md); fecha a lacuna "DrawImage no WebGPU" que o V3 precisa | [037-assets](implementacao/037-asset-core-drawimage.md), [038](implementacao/038-raster-drawimage.md), [029✓](implementacao/029✓-webgpu-renderer.md) (obrigatórias); [039](implementacao/039-canvas-drawimage.md) (parcial, DTO/frontend) |
| [047](implementacao/047-batch-instanciado-sprites.md) | Batch instanciado de sprites (040 §3.D camada 2) — `render.DrawSpriteBatch` + instâncias registradas render-only (`UpdateSpriteBatch`/`ConsumeSpriteBatchUpdates`), pipeline instanciado no WebGPU (flip-book + pulo senoidal + pulso "ola" no vertex shader, 1 draw call p/ 40k), raster segue oráculo com fórmula canônica, canvas dev com cap | 2026-07-07 | TODO — bloqueado por [046](implementacao/046-webgpu-texturas-quad-projetivo.md); marco V3.6 do [040](direcao/040-roadmap-visual-match-render.md); substrato genérico p/ torcida/bandeiras/partículas (usos são dados do jogo) | [046](implementacao/046-webgpu-texturas-quad-projetivo.md) (obrigatória; transitivamente 037-assets/038/029) |
| [048](implementacao/048-ci-determinismo-cross-arch.md) | CI de determinismo cross-arch (036 §4) — fixture dourada de checksum (`scene/testdata`, checkpoints tick 1/100/600), jobs arm64 (nativo ou QEMU — repo privado) + wasm no `ci.yml`, `forbidigo` barrando transcendentais em `scene`/`runtime`, auditoria de FMA (`float64(a*b)+c`) em `scene`+`math2d` | 2026-07-07 | TODO — **executável já**; adiantar = rede de segurança p/ [032✓](implementacao/032✓-ecs-sparse-set-storage.md)/[043✓](implementacao/043✓-runtime-rand-derive.md)/[049](implementacao/049-fisica-25d-engine.md) | nenhuma ([018✓](implementacao/018✓-ci-github-actions.md) e [016✓](implementacao/016✓-checksum-completo-ad9.md) satisfeitas) |
| [049](implementacao/049-fisica-25d-engine.md) | Física 2.5D na engine (036 §§1–3) — `Height`/`Spin`/`Ballistics` (arrasto quadrático, Magnus, quique, rolagem — só `+ - * / sqrt`), `Locomotion`/`MoveIntent` (curva por interpolação normalizada), ordenação de desenho por altura + `Projection` no `RenderCommands`; **passo 0 = benchmark** decidindo CCD×tick-rate (substeps vs swept) e storage (mapas em `scene` vs dedicado) | 2026-07-07 | TODO — fecha o portão do repo do jogo junto com o 048; regenera a fixture dourada (mudança de formato deliberada) | [042](implementacao/042-sombra-e-altura-football.md) Parte A (obrigatória); [048](implementacao/048-ci-determinismo-cross-arch.md) recomendado antes; ver nota sobre [032✓](implementacao/032✓-ecs-sparse-set-storage.md) |
| [050](implementacao/050-demo-2048.md) | Demo 2048 — primeira demo sem `scene.World`: estado puro em `runtime.StateSlot` (tipo-valor, sem `Clone`), RNG via `runtime.Rand` (043) — sem `rng.go`, o RNG vive no driver, input por borda (`JustPressed`+`Value`), `HoldSimulation` em todo tick, determinismo verificado por estado de jogo (checksum do `World` é constante com mundo vazio); hosts raster/canvas/wasm | 2026-07-07 | TODO — primeiro consumidor de produto do 043 | [043✓](implementacao/043✓-runtime-rand-derive.md) |
| [051✓](implementacao/051✓-wasm-debug-tools.md) | Ferramentas de debug no host WASM — painel unificado de tooling (`Options.DevTools`/`Options.RecordReplay`, toggle `` ` ``/`~`) dirigindo o driver in-process com a mesma superfície de controle do canvas (`Control`/`State`/`DebugLayers`); overlays vêm do `driver.Frame` (já renderizados no WebGPU), **zero mudança de engine**; botões de record/save/load vivem junto dos controles de debug; guarda `isEditableTarget` no input | 2026-07-07 | **DONE** (2026-07-08) — recupera as ferramentas de debug perdidas na migração canvas→host WASM; painel único começa oculto e alterna por `~`; stats compactas incluem `recording`; overlays de texto já renderizam no WebGPU/tess atual | [028✓](implementacao/028✓-wasm-host.md), [029✓](implementacao/029✓-webgpu-renderer.md) (satisfeitas) |
| [052✓](implementacao/052✓-render-interpolation.md) | Interpolacao de render para plataformas oficiais — runtime expoe `RenderContext.Alpha`, `scene.World` renderiza poses interpoladas e Breakout WASM valida suavizacao da bola rapida; **fora do canvas debug** | 2026-07-08 | **DONE** (2026-07-08) — opt-in inicial, alvo WASM e futuras plataformas oficiais; `platform/web/canvas` permanece ferramenta de debug sem interpolacao neste plano | [028✓](implementacao/028✓-wasm-host.md), [029✓](implementacao/029✓-webgpu-renderer.md) (satisfeitas) |
| [053](implementacao/053-demo-jogo-ritmo.md) | Demo de jogo de ritmo — highway de 4 pistas + holds + alvos de pointer, música WAV gerada, janelas de julgamento em ms, calibragem de latência, assets `DrawImage`, áudio via 037, estado puro em `StateSlot` e validação de rewind/scrub/replay por `MatchState` | 2026-07-08 | TODO — executar somente após 037-audio, 037-assets, 038, 039 e 050 concluídos | [037-audio](implementacao/037-audio.md), [037-assets](implementacao/037-asset-core-drawimage.md), [038](implementacao/038-raster-drawimage.md), [039](implementacao/039-canvas-drawimage.md), [050](implementacao/050-demo-2048.md) |
| [054✓](implementacao/054-benchmark-baseline-catalog.md) | Catalogação de benchmarks — baseline explícito + comparação estatística (benchstat) | 2026-07-08 | **DONE** (2026-07-08) — baselines por família em `benchmarks/baselines/`, benchmark reports em `benchmarks/runs/` (gitignored), metadados de provenance (commit SHA, Go version, GOMAXPROCS), comparação via benchstat com significância estatística (`~` = sem regressão), gate de regressão em testes, -profile decision com -count 10, e documentação de workflow | nenhuma (test-only dependency no `golang.org/x/perf`) |

> **Nota sobre numeração 037**: há dois planos com o número 037 (`037-audio.md`
> e `037-asset-core-drawimage.md`) por terem sido criados na mesma rodada. O
> sufixo no link (`-audio` / `-assets`) distingue os dois.

Status values: TODO | PARCIAL | DONE | SUBSTITUÍDO | WON'T DO | REFERÊNCIA (documento histórico/decisório, sem entregável de código próprio) | BLOCKED (com motivo) | REJECTED (com justificativa).

## Ordem de execução recomendada (trabalho pendente)

Esta ordem cobre só os itens TODO/PARCIAL — documentos de direção/review não se
"executam". Prioridades herdadas das rodadas de planejamento, reconciliadas numa
lista só.

**Reordenada em 2026-07-06** (passeio de quadrantes — o portão de criação do
repo do jogo, registrado nas correções do
[035](direcao/035-motor-de-partidas.md), define a sequência dos itens marcados
??; o D10 sai do 022 como passo avulso e o **023 passa a rodar antes do resto do
022**, para que o formato de keyframe/persistência nasça capturando o `rng`):

1. ✅ **[023](implementacao/023-recorder-keyframes-persistencia.md) (Fases 1–2)** — keyframes + seek unificado + `WriteTo`/`ReadRecording`/`VerifyRecording` com âncoras de checksum + demo integration; **Fase 3 (spill em disco) aguardando D5**
2. **V0: [037-assets](implementacao/037-asset-core-drawimage.md) ? [038](implementacao/038-raster-drawimage.md) ? [039](implementacao/039-canvas-drawimage.md)** — adiantada para antes do 022 (decisão de 2026-07-06): a demo expandida nasce no patamar pós-sprites. A **Parte A do [042](implementacao/042-sombra-e-altura-football.md)** (engine: `DrawEllipse`+`Projection`) é independente e pode rodar aqui em paralelo
3. ?? **[022](implementacao/022-football-expandido.md) (resto)** — IA, goleiro, zonas, sobre o seek novo; embute terminar o `Sensor` pendente do 014
4. **[042](implementacao/042-sombra-e-altura-football.md) Parte B** — fase 9 do 022 (sombras elípticas + altura visual da bola na demo, consumindo a engine da Parte A); logo após o 022
5. **[032✓](implementacao/032✓-ecs-sparse-set-storage.md)** — concluído em 2026-07-08; storage do World migrado para sparse sets densos ordenados, com Clone lazy para churn e entityCount cacheado; não é parte do portão
6. ✅ **[029✓](implementacao/029✓-webgpu-renderer.md)** — P3, depende de 028
7. ?? **[036](direcao/036-25d-engine-e-ci-determinismo.md) via [048](implementacao/048-ci-determinismo-cross-arch.md) + [049](implementacao/049-fisica-25d-engine.md)** — 2.5D na engine + CI de determinismo; fecha o portão — **o repo do jogo só é criado depois deste item**. O **[048](implementacao/048-ci-determinismo-cross-arch.md) não precisa esperar a posição 7**: é independente e vale adiantar (rede de determinismo p/ 032/043/049); o [049](implementacao/049-fisica-25d-engine.md) exige a Parte A do [042](implementacao/042-sombra-e-altura-football.md) e migra o protótipo do 042 para os componentes de engine

**Depende do 043 (DONE na árvore, 2026-07-07), independente do portão depois disso**: [050](implementacao/050-demo-2048.md) — demo 2048; primeira demo sem `scene.World` (estado puro em `StateSlot`, RNG via `runtime.Rand`/043 — sem `rng.go`, determinismo por estado de jogo pois o checksum do `World` é constante com mundo vazio); primeiro consumidor de produto do 043. Integração do 043 (Config.Seed/UpdateContext.Rand/driverSnapshot.rng + reseed no replay) verificada — sem blocker; o passo 0 do plano só revalida contra drift.

**Já concluídos** (não estão na lista de pendências):

- **[008✓](implementacao/008✓-runtime-pump-decomposicao-driver.md)**, **[014✓](implementacao/014✓-colisao-circle-rect.md)**, **[015✓](implementacao/015✓-auto-step-ad1.md)**, **[016✓](implementacao/016✓-checksum-completo-ad9.md)**, **[017✓](implementacao/017✓-bounds-restituicao-ad7.md)**, **[018✓](implementacao/018✓-ci-github-actions.md)**, **[019✓](implementacao/019✓-golden-frame-tests.md)** — núcleo runtime, colisores mistos, auto-step, checksum, bounds, CI e goldens entregues.
- **[024✓](implementacao/024✓-fix-bounce-rect-precedence.md)** e **[025✓](implementacao/025✓-mixed-collider-tests-benchmarks.md)** — precedência D1 corrigida e benchmarks mistos (2026-07-03).
- **[030✓](implementacao/030✓-public-api-surface-cleanup.md)** — API pública enxuta (2026-07-04).
- **[031✓](implementacao/031✓-transicoes-de-tela.md)** — transições de tela na UI (2026-07-05).
- **[020✓](implementacao/020✓-ui-manager-fundacoes.md)** e **[021✓](implementacao/021✓-demo-manager.md)** — fundações de UI de manager e demo Matchday (`cmd/demos/uimanager/`) entregues.
- **[026✓](implementacao/026✓-docs-drift-and-gomod.md)** — drift de AGENTS/README/go.mod corrigido.
- **[028✓](implementacao/028✓-wasm-host.md)** — host WASM de distribuição com Canvas2D provisório, input DOM/rAF e demo breakout no browser.
- **[043✓](implementacao/043✓-runtime-rand-derive.md)** — `runtime.Rand`, `Config.Seed`, `UpdateContext.Rand`, snapshot/rewind/scrub/replay, migração das demos e AD-10 entregues (2026-07-07).
- **[012](direcao/012-animacao-simulacao.md)** e **[013](direcao/013-animacao-ui.md)** (roadmaps DONE) — tweens determinísticos no `World` e animação de UI com clock de render.

Roadmaps de direção sem prioridade travada — atacar sob demanda, não por ordem:
[003](direcao/003-imgui.md) fase 6, [007](direcao/007-renderizacao.md) (agora
apontado para imagens-alvo concretas pelo [040](direcao/040-roadmap-visual-match-render.md),
que define as fases V0–V4 do visual de partida e os marcos demonstráveis).

**[037-audio](implementacao/037-audio.md) (áudio) é independente do portão** — não
bloqueia nem é bloqueado pelos itens ??. Encaixe natural: antes do
[023](implementacao/023-recorder-keyframes-persistencia.md) (para o formato de
keyframe já nascer com `audio.State`) ou a qualquer momento; o backend WASM do
áudio só existe depois do [028✓](implementacao/028✓-wasm-host.md).

**[052](implementacao/052-render-interpolation.md) (interpolacao de render) e
independente do portao e recomendado antes de polir demos rapidas em WASM** —
ele corrige a sensacao de arrasto/stutter entre ticks sem mudar simulacao,
checksum ou replay. O escopo e runtime/scene + Breakout WASM como validacao; o
`platform/web/canvas` continua fora por ser ferramenta de debug.

**[037-assets](implementacao/037-asset-core-drawimage.md) ?
[038](implementacao/038-raster-drawimage.md) ?
[039](implementacao/039-canvas-drawimage.md) ?
[041](implementacao/041-tipografia-fontes.md)** — trilha de assets/`DrawImage`
e tipografia (núcleo ? raster ? web canvas ? fontes); sequência própria.
**Desde 2026-07-06 a V0 (037-assets→038→039) deixou de ser totalmente
independente do portão**: o [022](implementacao/022-football-expandido.md)
foi adiado para depois dela (fase 9 visual, plano
[042](implementacao/042-sombra-e-altura-football.md) consome `DrawImage`).
O [041](implementacao/041-tipografia-fontes.md) segue fora do portão.
O [044](implementacao/044-texturas-dinamicas.md) (texturas dinâmicas, 040
§3.A) encaixa logo após o 039 — destrava metade do marco V2 (heatmap como
textura) sem depender de GPU. O
[045](implementacao/045-transform-afim.md) (Tier 1, transform afim) tem as
primitivas independentes de tudo e o `DrawImage` transformado atrás do 038;
junto com o 036 e o 044 fecha o marco V2 do 040 (sombra direcional com
skew = 042 + 045).
Encaixe natural: antes do [021✓](implementacao/021✓-demo-manager.md) (a demo de
manager quer ícones/retratos) e antes do
[036](direcao/036-25d-engine-e-ci-determinismo.md) (sprites 2.5D); os
renderers de [028✓](implementacao/028✓-wasm-host.md)/[029✓](implementacao/029✓-webgpu-renderer.md)
consomem o mesmo registry de handles quando chegarem.

**[053](implementacao/053-demo-jogo-ritmo.md)** — demo de ritmo planejada para
depois de [037-audio](implementacao/037-audio.md), V0 de assets
([037-assets](implementacao/037-asset-core-drawimage.md) ->
[038](implementacao/038-raster-drawimage.md) ->
[039](implementacao/039-canvas-drawimage.md)) e
[050](implementacao/050-demo-2048.md). Ela pressiona sincronização perceptível
entre áudio/input/render, janelas de julgamento em milissegundos, calibragem de
latência, `StateSlot` sem `scene.World`, áudio em rewind/scrub e validação no
host WASM.

## Notas de dependência

- **025 depende de 024**: 024 corrige a precedência de colisor no bounce e já
  adiciona os testes de bounce com rect; 025 assume esses nomes de teste
  existentes e testa o comportamento corrigido.
- **027 ? 030**: o 027 propunha só deprecar por doc comment; o 030 o absorve
  com remoção/privatização direta (módulo pré-v1 sem consumidor externo,
  precedente AD-3). O teste de equivalência do 025 continua vivo via ponte
  `export_test.go` descrita no 030.
- **030 depende da árvore de 012/013**: o redesenho de `ToastOptions` e os
  doc comments de posicionamento em `ui/` tocam código que existe apenas na
  branch `feat/animation` (não commitado sobre `66370c9` na data do plano).
- **029 depende de 028**: o WebGPU implementa a interface `renderer` que o
  host WASM de 028 define; 028 valida loop/input/build antes de GPU entrar.
- **021 depende de 020**: a demo de manager (021) consome os widgets/temas
  que 020 constrói em `ui/`/`render/`.
- **014/017 desbloqueiam parte de 022**: o football expandido (022) pode usar os
  lados abertos/restituição opcional entregues em 017 para tratar bola fora e
  amortecimento de paredes sem hacks no demo.
- **036 fecha o portão do repo do jogo** (2026-07-06): o repo privado do
  jogo só é criado após D10+`Derive`, 023, 022, 028, 029 e 036. Web-only no
  lançamento; nativo é futuro e não bloqueia. Detalhe completo nas correções
  de 2026-07-06 do [035](direcao/035-motor-de-partidas.md).
- **037-audio × 023**: o `driverSnapshot` ganha `audio.State` no 037-audio;
  se o 023 (keyframes/persistência) rodar depois, o formato serializado deve
  incluir o campo novo. Se o 023 rodar antes, o 037-audio adiciona o campo
  nos dois lugares.
- **037-audio × 028**: o backend Web Audio do canvas (037-audio) é a
  especificação de referência do backend do host WASM — ao executar o 028,
  portar o consumo (`ConsumeAudio` no rAF + `audio.js` via `syscall/js`) ou
  abrir plano curto.
- **038 depende de 037-assets**: o 038 implementa `DrawImage` no raster usando
  `render.DrawImage`, `asset.Manifest` e `asset/pixel` criados no 037-assets.
- **053 depende de 037-audio/037-assets/038/039/050**: a demo de ritmo assume
  áudio, assets e `DrawImage` funcionando nos hosts interativos, além do padrão
  de estado puro/rewind da demo 2048. Se o sink de áudio do WASM ainda não
  existir quando o 053 for executado, o plano manda portar o consumo mínimo de
  `Driver.ConsumeAudio` antes de validar a demo no host oficial.
- **043 antes do 023 (obrigatório)**: o formato de keyframe/persistência do
  023 deve nascer capturando o `rng` do driver (1 uint64 no
  `driverSnapshot`) — retrofit depois seria mudança de formato. A sequência
  do splitmix64 é contrato congelado por KAT
  (`runtime/random_test.go`); nunca "refatorar" o gerador após o primeiro
  replay persistido.
- **044 depende da V0 e revisa uma premissa do 039**: o `tintCache` do
  frontend assume textura imutável por execução (nota de manutenção do
  039); o 044 introduz conteúdo mutável e adiciona a invalidação por
  id/versão. Conteúdo de textura dinâmica é render-only — nunca entra em
  snapshot/checksum/recording; os dados de origem (ex.: contadores do
  heatmap) vivem no World/estado do jogo.
- **042 é a engine do 2.5D visual**: Parte A (engine — `render.DrawEllipse`
  ponta a ponta em raster/adapter/JS + `render.Projection`) não depende de
  nada e materializa a linha do concept do
  [040](direcao/040-roadmap-visual-match-render.md) §1 que estava sem plano.
  Parte B (demo) depende do [022](implementacao/022-football-expandido.md)
  fases 1–4 (loft ancora no `kick()`). O que fica para o
  [036](direcao/036-25d-engine-e-ci-determinismo.md): física de projétil
  (`z` como estado de engine), ordenação de desenho por altura e a
  tesselação WebGPU (nota deixada no
  [029✓](implementacao/029✓-webgpu-renderer.md) pelo passo A6 do 042).
- **046 depende de 037-assets/038/029 e complementa o V3**: o pipeline
  texturizado do WebGPU (que o V3 precisa para sprites) nasce no 046 junto
  com mipmaps/aniso e o quad projetivo (`DrawImageQuad`, caminho B do 040
  §3.C — decisão de 2026-07-07 de implementar os dois caminhos). O 044
  executado depois do 046 deve regenerar mips no `ConsumeTextureUpdates`
  (nota de manutenção do 046).
- **051 não toca engine** (2026-07-07): o painel de debug do host WASM
  dirige o `runtime.Driver` pela superfície pública que o servidor do
  canvas já usa (`Control`/`State`/`ControlCommand`/`scene.DebugLayers`);
  os overlays já são dobrados por `driver.Frame` e desenhados pelo WebGPU
  do [029✓](implementacao/029✓-webgpu-renderer.md). Recupera as ferramentas
  que só existiam no canvas (AD-4). Overlays de texto (IDs, HUD) dependem
  de texto no WebGPU — [041](implementacao/041-tipografia-fontes.md);
  geometria (colisores/grid/velocidade) funciona já.
- **052 e contrato de engine, nao backend**: a interpolacao deve nascer em
  `runtime`/`scene` e produzir `render.Command` ja suavizado para WASM e futuras
  plataformas oficiais. `platform/web/canvas` e `internal/render/adapter/web`
  ficam fora porque o canvas e ferramenta de debug, nao alvo de distribuicao.
- **048 antes de 032/049 (recomendado)**: a fixture dourada de checksum é
  a prova de preservação da migração de storage (032) e o protocolo de
  mudança de formato quando o 049 adicionar componentes (regeneração em
  commit próprio). 048 é independente de tudo; 049 exige 042 Parte A e
  decide CCD/storage por benchmark no passo 0 (questões abertas do 036).
- **047 depende de 046 e é o substrato do "estádio vivo"**: o pipeline
  instanciado (torcida/bandeiras/partículas, marco V3.6 do 040 §3.D) monta
  sobre o registry de texturas, o blend premultiplicado e os runs do 046.
  Conteúdo de batch é render-only (nunca em snapshot/checksum); blend
  aditivo (clarão de sinalizador) fica atrás da decisão Tier 3 do 040 §4.
- **028/029 revisam o AD-4**: decisão do mantenedor em 2026-07-02 — backend
  gráfico passa de WebGL para **WebGPU** (browser via `syscall/js`; nativo via
  `wgpu-native` no futuro). O passo 1 de 028 registra o adendo em
  [010](direcao/010-decisoes-arquiteturais.md).
- Nota de contexto: o pacing/borrado percebido no web canvas (frames pintados
  na chegada do WebSocket, fora do vsync) foi analisado e **descartado como
  não-valendo-correção** pelo mantenedor — o canvas é ferramenta de debug
  (AD-4 em [010](direcao/010-decisoes-arquiteturais.md)); o caminho de
  distribuição é o host WASM de 028/029.

## Achados considerados e rejeitados (não re-auditar)

- **"Normal invertido nas faces esquerda/topo de `circleVsRect`"**: falso
  positivo. A convenção é `Contact.Normal` apontando A?B e o resolvedor move
  A em `-Normal` (`scene/collision.go:435-438`) — normal correta para o caso
  descrito.
- **"Checksum incompleto" como achado novo**: é o AD-9, já planejado em
  [016✓](implementacao/016✓-checksum-completo-ad9.md).
- **Cap no inbox do web canvas** (`platform/web/canvas/server.go:90` —
  `pendingEvents`/`pendingCmds` sem limite): confirmado mas não vale plano
  agora — drenado a cada tick (~16ms), servidor binda em `127.0.0.1`,
  ferramenta de dev por decisão AD-4. Reavaliar se o canvas ganhar exposição
  além de localhost.
- **`.golangci.yml`/lint local**: baixa alavancagem para dev solo; 018 já
  cobre `gofmt`/`go vet` no pipeline de CI. Reavaliar com mais contribuidores.
- **`.editorconfig`**: ruído — `gofmt` governa formatação em Go.
- **Micro-otimização de `Clamp` no narrow phase circle×rect**: custo ~ns,
  contra a filosofia "medir antes de otimizar". Só com profile mostrando o
  caminho quente.
- **Serialização por cliente no broadcast do canvas / multi-cliente**:
  decidido em AD-4 (ferramenta de dev, 1 cliente típico) — despriorizado pelo
  mantenedor.

## Cobertura da última auditoria (skill `improve`, 2026-07-02)

Cobertura (nível standard): correção/determinismo (muito a fundo no código
novo de `feat/rect-colliders`), segurança+concorrência (muito a fundo em
`platform/web/canvas` e `platform/raster`), performance+testes (médio,
focado no código novo), dívida/arquitetura/deps/DX/docs (médio). Direção
coberta por reconciliação com [011](direcao/011-visao-diferenciais.md).

**Não auditado**: os guias em `docs/` (runtime.md, ui.md, scrubbing.md —
podem ter drift pós-rect-colliders), o frontend JS embutido do canvas
(`index.html`/`app.js`), e os demos em profundidade de gameplay.

Limpo (sem achados): layering (nenhuma violação do grafo de dependências),
races (locks do hub corretos), vazamento de goroutines, bind de rede
(127.0.0.1), `World.Clone`/`Despawn` cobrindo os mapas novos, ordenação
determinística nos caminhos novos de render/debug.
