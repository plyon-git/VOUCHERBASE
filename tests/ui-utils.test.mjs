// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
import test from 'node:test';import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const text=await readFile(new URL('../apps/api-go/web/ui-utils.js',import.meta.url),'utf8');
const {cents,money,safeLink,percent,dateOnly}=await import('data:text/javascript;base64,'+Buffer.from(text).toString('base64'));
test('exact cents',()=>{assert.equal(cents('1234.56'),123456);assert.equal(cents('0'),0);assert.equal(cents('1.2'),120);});
test('bad amounts',()=>{for(const v of ['NaN','1.001','-1','','1e8'])assert.throws(()=>cents(v));});
test('unknown is not zero',()=>{assert.equal(money(null),'Unverified');assert.equal(money(0),'$0.00');});
test('external links',()=>{assert.equal(safeLink('javascript:alert(1)'),null);assert.equal(safeLink('https://user:pass@example.com'),null);assert.ok(safeLink('https://example.com/source'));});
test('ratios',()=>{assert.equal(percent(null),'N/A');assert.equal(percent(550),'5.50%');});
test('date',()=>assert.match(dateOnly(),/^\d{4}-\d{2}-\d{2}$/));
