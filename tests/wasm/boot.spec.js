const { test, expect } = require('@playwright/test');

const goldenPort = process.env.WASM_GOLDEN_PORT || '8090';
const minimumFadeObservationMillis = 100;

for (const scenario of [
  {
    name: 'landscape',
    viewport: { width: 320, height: 240 },
    poster: 'arasy-poster.svg',
    excludedPoster: 'arasy-poster-stacked.svg',
  },
  {
    name: 'portrait',
    viewport: { width: 240, height: 320 },
    poster: 'arasy-poster-stacked.svg',
    excludedPoster: 'arasy-poster.svg',
  },
]) {
  test(`keeps the ${scenario.name} poster until the default splash is opaque`, async (
    { page },
    testInfo,
  ) => {
    const expectedRenderer = testInfo.project.name;
    const messages = [];
    const posterRequests = [];
    page.on('console', message => messages.push(message.text()));
    page.on('pageerror', error => messages.push(`page error: ${error.message}`));
    page.on('request', request => {
      const filename = new URL(request.url()).pathname.split('/').pop();
      if (filename === scenario.poster || filename === scenario.excludedPoster) {
        posterRequests.push(filename);
      }
    });

    await page.setViewportSize(scenario.viewport);
    await page.addInitScript(renderer => {
      const probe = {
        rendererReadyAt: 0,
        firstFrameAt: 0,
        bootPresentAtFirstFrame: false,
        canvasVisibilityAtFirstFrame: '',
      };
      window.arasyBootProbe = probe;

      const originalInfo = console.info.bind(console);
      console.info = (...args) => {
        const message = args.map(String).join(' ');
        if (message.includes(`using ${renderer} renderer`)) {
          probe.rendererReadyAt = performance.now();
        }
        originalInfo(...args);
      };

      let firstFrame = false;
      Object.defineProperty(window, 'arasyFirstFrame', {
        configurable: true,
        get: () => firstFrame,
        set: value => {
          if (value === true && firstFrame === false) {
            const canvas = document.getElementById('ar-canvas');
            probe.firstFrameAt = performance.now();
            probe.bootPresentAtFirstFrame = document.getElementById('ar-boot') !== null;
            probe.canvasVisibilityAtFirstFrame =
              canvas === null ? '' : getComputedStyle(canvas).visibility;
          }
          firstFrame = value;
        },
      });
    }, expectedRenderer);

    let releaseWASM;
    const wasmGate = new Promise(resolve => {
      releaseWASM = resolve;
    });
    await page.route('**/app.wasm', async route => {
      await wasmGate;
      await route.continue();
    });

    await page.goto(
      `http://127.0.0.1:${goldenPort}/?renderer=${expectedRenderer}&fixture=boot`,
    );

    const boot = page.locator('#ar-boot');
    const canvas = page.locator('#ar-canvas');
    try {
      await expect(boot).toHaveCSS('visibility', 'visible');
      await expect(canvas).toHaveCSS('visibility', 'hidden');
      expect(await page.locator('#ar-poster').evaluate(image => {
        return new URL(image.currentSrc).pathname.split('/').pop();
      })).toBe(scenario.poster);
      expect(posterRequests).toContain(scenario.poster);
      expect(posterRequests).not.toContain(scenario.excludedPoster);
      expect(await page.evaluate(() => window.arasyFirstFrame === true)).toBe(false);
    } finally {
      releaseWASM();
    }

    await expect.poll(
      () => messages.join('\n'),
      { message: `expected ${expectedRenderer} renderer; browser log:\n${messages.join('\n')}` },
    ).toContain(`using ${expectedRenderer} renderer`);
    await page.waitForFunction(() => window.arasyBootFadeDone === true);

    const probe = await page.evaluate(() => window.arasyBootProbe);
    expect(probe.rendererReadyAt).toBeGreaterThan(0);
    expect(probe.firstFrameAt - probe.rendererReadyAt)
      .toBeGreaterThanOrEqual(minimumFadeObservationMillis);
    expect(probe.bootPresentAtFirstFrame).toBe(true);
    expect(probe.canvasVisibilityAtFirstFrame).toBe('hidden');
    await expect(boot).toHaveCount(0);
    await expect(canvas).toHaveCSS('visibility', 'visible');
    await expect(canvas).toHaveAttribute('data-arasy-ready', expectedRenderer);
  });
}
