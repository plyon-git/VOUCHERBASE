// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
// Use only a disposable, seeded Compose stack. Never record or print access tokens.
import { chromium } from 'playwright';
import fs from 'node:fs';
import assert from 'node:assert/strict';

const vars = Object.fromEntries(fs.readFileSync('.env', 'utf8').split('\n')
  .filter(line => line.includes('='))
  .map(line => [line.slice(0, line.indexOf('=')), line.slice(line.indexOf('=') + 1)]));
assert.ok(vars.VB_API_TOKEN, 'The disposable test token must be configured');
fs.mkdirSync('artifacts', { recursive: true });
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
const errors = [];
page.on('pageerror', error => errors.push(error.message));

async function login() {
  await page.locator('#api-token').fill(vars.VB_API_TOKEN);
  await page.locator('#login-form button').click();
  await page.locator('#workspace').waitFor({ state: 'visible' });
  assert.equal(await page.locator('#api-token').inputValue(), '');
}
async function logout() {
  assert.equal(await page.locator('#logout').isVisible(), true, 'Sign out must remain accessible');
  await page.locator('#logout').click();
  await page.locator('#login').waitFor({ state: 'visible' });
  assert.equal(await page.locator('#workspace').isVisible(), false);
  assert.equal(await page.locator('#analysis-results').isVisible(), false);
  assert.equal(await page.evaluate(() => localStorage.length), 0, 'No localStorage credentials');
  assert.equal(await page.evaluate(() => sessionStorage.length), 0, 'No sessionStorage credentials');
}

try {
  await page.goto('http://localhost:8080');
  await login();
  await page.screenshot({ path: 'artifacts/overview.png', fullPage: true });
  await page.locator('nav button[data-page=analyze]').click();
  await page.locator('#load-demo').click();
  await page.locator('#analyze-submit').click();
  await page.locator('#analysis-results').waitFor({ state: 'visible', timeout: 60000 });
  assert.ok((await page.locator('#result-warnings').textContent()).includes('SYNTHETIC'));
  await page.locator('#replay-result').click();
  await page.waitForFunction(() => document.querySelector('#status').textContent.includes('Replay matched'));
  await page.screenshot({ path: 'artifacts/desktop.png', fullPage: true });

  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 2), false, 'Mobile horizontal overflow');
  await page.screenshot({ path: 'artifacts/mobile.png', fullPage: true });
  await logout();

  // A fresh mobile sign-in must also expose every workspace view and Sign out.
  await login();
  for (const view of ['overview', 'analyze', 'portfolio', 'evidence', 'imports']) {
    await page.locator(`nav button[data-page=${view}]`).click();
    await page.locator(`#page-${view}`).waitFor({ state: 'visible' });
  }
  await logout();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await login();
  await logout();
  assert.deepEqual(errors, []);
  console.log('Browser passed: desktop and mobile sign-in/sign-out, all five views, real demo analysis, replay, screenshots, mobile fit and no JavaScript errors.');
} catch (error) {
  await page.screenshot({ path: 'artifacts/browser-failure.png', fullPage: true });
  throw error;
} finally {
  await browser.close();
}
