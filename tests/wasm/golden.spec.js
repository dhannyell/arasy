const { test, expect } = require('@playwright/test');

test('renders the canonical WASM frame', async ({ page }, testInfo) => {
  const expectedRenderer = testInfo.project.name;
  const messages = [];
  page.on('console', message => messages.push(message.text()));
  page.on('pageerror', error => messages.push(`page error: ${error.message}`));

  await page.goto(`http://127.0.0.1:8090/?renderer=${expectedRenderer}`);

  await expect.poll(
    () => messages.join('\n'),
    { message: `expected ${expectedRenderer} renderer; browser log:\n${messages.join('\n')}` },
  ).toContain(`using ${expectedRenderer} renderer`);

  const canvas = page.locator('#ff-canvas');
  await expect(canvas).toHaveAttribute('data-frameforge-ready', expectedRenderer);
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
  await expect(canvas).toHaveScreenshot('frame.png');
});
