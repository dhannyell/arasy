const { test, expect } = require('@playwright/test');
const goldenPort = process.env.WASM_GOLDEN_PORT || '8090';

// Canvas2D draws text with the system font and approximates LensRain, so
// these goldens match only the platform that recorded them.
function skipCanvas2DOnLinux(testInfo) {
  test.skip(
    testInfo.project.name === 'canvas2d' && process.platform === 'linux',
    'Canvas2D golden recorded on Windows.',
  );
}

async function expectFixtureScreenshot(page, testInfo, fixture, screenshot, options = '') {
  const expectedRenderer = testInfo.project.name;
  const messages = [];
  page.on('console', message => messages.push(message.text()));
  page.on('pageerror', error => messages.push(`page error: ${error.message}`));

  const fixtureParam = fixture ? `&fixture=${fixture}` : '';
  await page.goto(`http://127.0.0.1:${goldenPort}/?renderer=${expectedRenderer}${fixtureParam}${options}`);

  await expect.poll(
    () => messages.join('\n'),
    { message: `expected ${expectedRenderer} renderer; browser log:\n${messages.join('\n')}` },
  ).toContain(`using ${expectedRenderer} renderer`);

  const canvas = page.locator('#ar-canvas');
  await expect(canvas).toHaveAttribute('data-Arasy-ready', expectedRenderer);
  // The shell removes its boot overlay only after the canvas has presented a
  // handoff-ready frame. Waiting for the completion flag keeps the overlay out
  // of the screenshot.
  await page.waitForFunction(() => window.arasyBootFadeDone === true);
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
  try {
    await expect(canvas).toHaveScreenshot(screenshot);
  } catch (error) {
    throw new Error(`${error.message}\n\nBrowser log:\n${messages.join('\n')}`);
  }
}

test('renders the canonical WASM frame', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(page, testInfo, '', 'frame.png');
});

test('renders particle backend semantics', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(
    page,
    testInfo,
    'particles',
    'particles.png',
  );
});

test('renders sprite texture arrays and 2d layer views', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(
    page,
    testInfo,
    'sprite-array',
    'sprite-array.png',
  );
});

test('renders retained sprite and text batches', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(
    page,
    testInfo,
    'retained-batches',
    'retained-batches.png',
  );
});

test('renders isometric batch and particle conformance', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(
    page,
    testInfo,
    'view',
    'view.png',
  );
});

test('renders layer parallax across retained and streaming content', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(
    page,
    testInfo,
    'parallax',
    'parallax.png',
  );
});

test('renders source flips before rotation in streaming and retained sprites', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(page, testInfo, 'flip', 'flip.png');
});

test('renders two camera passes with independent viewports', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(page, testInfo, 'passes', 'passes.png');
});

test('renders two camera passes without WebGPU MSAA', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'webgpu', 'MSAA applies only to WebGPU.');
  await expectFixtureScreenshot(page, testInfo, 'passes', 'passes-msaa-off.png', '&msaa=off');
});

test('renders the fixed post-effect chain', async ({ page }, testInfo) => {
  skipCanvas2DOnLinux(testInfo);
  await expectFixtureScreenshot(page, testInfo, 'post', 'post.png');
});

test('renders Halftone and FocusBlur', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(page, testInfo, 'post-stylize', 'post-stylize.png');
});

test('renders RTT through sprite, textured, retained, and post paths', async ({ page }, testInfo) => {
  skipCanvas2DOnLinux(testInfo);
  await expectFixtureScreenshot(page, testInfo, 'rtt', 'rtt.png');
});

test('renders RTT while preserving the sprite texture array', async ({ page }, testInfo) => {
  skipCanvas2DOnLinux(testInfo);
  await expectFixtureScreenshot(page, testInfo, 'rtt-array', 'rtt-array.png');
});
