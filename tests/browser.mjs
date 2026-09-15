// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
// Run with a disposable seeded Compose stack and npm-installed Playwright.
import {chromium} from 'playwright';import fs from 'node:fs';import assert from 'node:assert/strict';
const vars=Object.fromEntries(fs.readFileSync('.env','utf8').split('\n').filter(x=>x.includes('=')).map(x=>[x.slice(0,x.indexOf('=')),x.slice(x.indexOf('=')+1)]));
const browser=await chromium.launch({headless:true});const errors=[];const page=await browser.newPage({viewport:{width:1440,height:1000}});page.on('pageerror',e=>errors.push(e.message));
try{
 await page.goto('http://localhost:8080');await page.locator('#api-token').fill(vars.VB_API_TOKEN);await page.locator('#login-form button').click();await page.locator('#workspace').waitFor({state:'visible'});
 await page.locator('nav button[data-page=analyze]').click();await page.locator('#load-demo').click();await page.locator('#analyze-submit').click();await page.locator('#analysis-results').waitFor({state:'visible',timeout:60000});
 assert.ok((await page.locator('#result-warnings').textContent()).includes('SYNTHETIC'));
 await page.locator('#replay-result').click();await page.waitForFunction(()=>document.querySelector('#status').textContent.includes('Replay matched'));
 fs.mkdirSync('artifacts',{recursive:true});await page.screenshot({path:'artifacts/desktop.png',fullPage:true});
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:'artifacts/mobile.png',fullPage:true});
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+2),false,'mobile horizontal overflow');
 await page.locator('#logout').click();await page.locator('#login').waitFor({state:'visible'});
 assert.deepEqual(errors,[]);console.log('Browser: login, demo analysis, replay, screenshots, mobile fit and logout passed.');
}finally{await browser.close();}
