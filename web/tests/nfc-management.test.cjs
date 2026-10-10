const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const source = fs.readFileSync(path.join(__dirname, '../src/app/dashboard/nfc/page.tsx'), 'utf8');

test('NFC page has a clear unconfigured state and requires a published card to configure', () => {
  assert.match(source, /No NFC tags yet/);
  assert.match(source, /Publish a digital card before adding an NFC tag/);
  assert.match(source, /card\.status===\"published\"/);
  assert.match(source, /Choose one of your published cards/);
});

test('configured NFC state displays the card, status, and target URL with copy and open actions', () => {
  assert.match(source, /Connected Card/);
  assert.match(source, /tag\.card_name/);
  assert.match(source, /tag\.status===\"active\"\?t\(\"Active\"\):t\(\"Inactive\"\)/);
  assert.match(source, /tag\.redirect_url/);
  assert.match(source, /navigator\.clipboard\.writeText\(url\)/);
  assert.match(source, /function testLink\(url:string\)/);
});

test('NFC saves create or update through the customer API and reload persisted configuration', () => {
  assert.match(source, /editing\?`\/user\/nfc\/\$\{editing\.id\}`:\"\/user\/nfc\"/);
  assert.match(source, /method:editing\?\"PUT\":\"POST\"/);
  assert.match(source, /if\(editing\).*?tags\.reload\(\)/s);
  assert.match(source, /tags\.reload\(\)/);
  assert.match(source, /catch\(err\)\s*\{\s*setError/);
});

