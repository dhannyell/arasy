import { mkdirSync } from 'node:fs';
import { spawnSync } from 'node:child_process';

const output = 'output/wasm/golden/app.wasm';
mkdirSync('output/wasm/golden', { recursive: true });

const result = spawnSync(
  'go',
  ['build', '-o', output, './cmd/wasmgolden/wasm'],
  {
    env: { ...process.env, GOOS: 'js', GOARCH: 'wasm' },
    stdio: 'inherit',
    shell: process.platform === 'win32',
  },
);

if (result.error) {
  throw result.error;
}
if (result.status !== 0) {
  process.exit(result.status ?? 1);
}
