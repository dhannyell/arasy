const { defineConfig, devices } = require('@playwright/test');

const webGPUArgs = [
  '--enable-unsafe-webgpu',
  '--use-webgpu-adapter=swiftshader',
  '--enable-dawn-features=allow_unsafe_apis',
  '--disable-dawn-features=use_dxc',
  '--enable-webgpu-developer-features',
  '--use-gpu-in-tests',
  '--enable-accelerated-2d-canvas',
  '--enable-unsafe-swiftshader',
];

module.exports = defineConfig({
  testDir: '.',
  outputDir: 'test-results',
  timeout: 30_000,
  expect: {
    timeout: 10_000,
    toHaveScreenshot: {
      animations: 'disabled',
      maxDiffPixelRatio: 0.005,
      threshold: 0.2,
    },
  },
  snapshotPathTemplate: 'platform/wasm/testdata/golden/{projectName}/{arg}{ext}',
  webServer: {
    command: 'go run ./cmd/wasmserve -dir output/wasm/golden -addr 127.0.0.1:8090',
    url: 'http://127.0.0.1:8090',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [
    {
      name: 'canvas2d',
      use: {
        ...devices['Desktop Chrome'],
        browserName: 'chromium',
        channel: 'chromium',
        deviceScaleFactor: 1,
        viewport: { width: 320, height: 240 },
      },
    },
    {
      name: 'webgpu',
      use: {
        ...devices['Desktop Chrome'],
        browserName: 'chromium',
        channel: 'chromium',
        deviceScaleFactor: 1,
        viewport: { width: 320, height: 240 },
        launchOptions: {
          args: webGPUArgs,
        },
      },
    },
  ],
});
