const { test, expect } = require('@playwright/test');
const goldenPort = process.env.WASM_GOLDEN_PORT || '8090';

async function expectFixtureScreenshot(page, testInfo, fixture, screenshot) {
  const expectedRenderer = testInfo.project.name;
  const messages = [];
  page.on('console', message => messages.push(message.text()));
  page.on('pageerror', error => messages.push(`page error: ${error.message}`));

  const fixtureParam = fixture ? `&fixture=${fixture}` : '';
  await page.goto(`http://127.0.0.1:${goldenPort}/?renderer=${expectedRenderer}${fixtureParam}`);

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

test('renders the fixed post-effect chain', async ({ page }, testInfo) => {
  await expectFixtureScreenshot(page, testInfo, 'post', 'post.png');
});
