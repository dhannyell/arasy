const { test, expect } = require('@playwright/test');
const goldenPort = process.env.WASM_GOLDEN_PORT || '8090';

async function expectFixtureScreenshot(page, fixture, screenshot, options = '') {
  const messages = [];
  page.on('console', message => messages.push(message.text()));
  page.on('pageerror', error => messages.push(`page error: ${error.message}`));

  const fixtureParam = fixture ? `&fixture=${fixture}` : '';
  await page.goto(`http://127.0.0.1:${goldenPort}/?${fixtureParam}${options}`);

  await expect.poll(
    () => messages.join('\n'),
    { message: `expected webgpu renderer; browser log:\n${messages.join('\n')}` },
  ).toContain('using webgpu renderer');

  const canvas = page.locator('#ar-canvas');
  await expect(canvas).toHaveAttribute('data-arasy-ready', 'webgpu');
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

test('renders the canonical WASM frame', async ({ page }) => {
  await expectFixtureScreenshot(page, '', 'frame.png');
});

test('renders particle backend semantics', async ({ page }) => {
  await expectFixtureScreenshot(page, 'particles',
    'particles.png',
  );
});

test('renders sprite texture arrays and 2d layer views', async ({ page }) => {
  await expectFixtureScreenshot(page, 'sprite-array',
    'sprite-array.png',
  );
});

test('renders retained sprite and text batches', async ({ page }) => {
  await expectFixtureScreenshot(page, 'retained-batches',
    'retained-batches.png',
  );
});

test('renders isometric batch and particle conformance', async ({ page }) => {
  await expectFixtureScreenshot(page, 'view',
    'view.png',
  );
});

test('renders layer parallax across retained and streaming content', async ({ page }) => {
  await expectFixtureScreenshot(page, 'parallax',
    'parallax.png',
  );
});

test('renders source flips before rotation in streaming and retained sprites', async ({ page }) => {
  await expectFixtureScreenshot(page, 'flip', 'flip.png');
});

test('renders two camera passes with independent viewports', async ({ page }) => {
  await expectFixtureScreenshot(page, 'passes', 'passes.png');
});

test('renders two camera passes without WebGPU MSAA', async ({ page }) => {
  await expectFixtureScreenshot(page, 'passes', 'passes-msaa-off.png', '&msaa=off');
});

test('renders a retained text batch after the uniform arena grows', async ({ page }) => {
  await expectFixtureScreenshot(page, 'arena-growth', 'arena-growth.png');
});

test('renders the fixed post-effect chain', async ({ page }) => {
  await expectFixtureScreenshot(page, 'post', 'post.png');
});

test('renders Halftone and FocusBlur', async ({ page }) => {
  await expectFixtureScreenshot(page, 'post-stylize', 'post-stylize.png');
});

test('renders RTT through sprite, textured, retained, and post paths', async ({ page }) => {
  await expectFixtureScreenshot(page, 'rtt', 'rtt.png');
});

test('renders RTT while preserving the sprite texture array', async ({ page }) => {
  await expectFixtureScreenshot(page, 'rtt-array', 'rtt-array.png');
});
