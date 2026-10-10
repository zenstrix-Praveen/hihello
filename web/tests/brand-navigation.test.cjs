const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..');
const read = relative => fs.readFileSync(path.join(root, relative), 'utf8');

test('shared brand assets use the supplied DBIZCARD logo and an emblem favicon', () => {
  const component = read('src/components/brand/BrandLogo.tsx');
  const layout = read('src/app/layout.tsx');
  assert.match(component, /src="\/logo\.png"/);
  assert.match(component, /width=\{1942\}/);
  assert.match(component, /height=\{809\}/);
  assert.ok(fs.existsSync(path.join(root, 'public/logo.png')));
  assert.ok(fs.existsSync(path.join(root, 'public/brand/dbizcard-icon.png')));
  assert.match(layout, /DBIZCARD \| Digital Business Cards/);
  assert.match(layout, /dbizcard-icon\.png/);
});

test('public navigation follows the specified desktop sizing and responsive menu breakpoint', () => {
  const header = read('src/components/public/PublicHeader.tsx');
  const languageSelector = read('src/components/locale/CustomerLanguageSwitcher.tsx');
  assert.match(header, /min-h-\[71px\]/);
  assert.match(header, /max-w-7xl/);
  assert.match(header, /lg:px-8/);
  assert.match(header, /gap-6/);
  assert.match(header, /min-\[1280px\]:flex/);
  assert.match(header, /min-\[1280px\]:hidden/);
  assert.match(header, /h-10 items-center gap-1\.5[\s\S]*text-sm/);
  assert.match(header, /h-12 w-\[190px\]/);
  assert.match(languageSelector, /min-h-10/);
});

test('frontend source has no remaining HiHello platform branding', () => {
  const source = path.join(root, 'src');
  const files = [];
  function walk(directory) {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) walk(file);
      else if (/\.[jt]sx?$/.test(file)) files.push(file);
    }
  }
  walk(source);
  const branded = files.filter(file => {
    const source = fs.readFileSync(file, 'utf8').replace(/\/hihello(?:\|dbizcard)?\/gi/g, '');
    return /hihello/i.test(source);
  });
  assert.deepEqual(branded, [], 'Replace old brand copy in frontend source');
});
