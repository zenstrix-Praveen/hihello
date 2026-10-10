const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');

const src = path.join(__dirname, '../src');
function loadTS(relative) {
  const filename = path.join(src, relative);
  const code = ts.transpileModule(fs.readFileSync(filename, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText;
  const mod = { exports: {} };
  new Function('require', 'module', 'exports', code)((name) => {
    if (name.includes('plan-currency-messages')) return loadTS('lib/plan-currency-messages.ts');
    throw new Error(`Unexpected localization dependency: ${name}`);
  }, mod, mod.exports);
  return mod.exports;
}

test('every literal customer and public translation key has an Arabic entry', () => {
  const directories = ['app', 'components'];
  const files = [];
  function walk(directory) {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) {
        if (entry.name === 'admin') continue;
        walk(file);
      } else if (/\.tsx?$/.test(file)) files.push(file);
    }
  }
  directories.forEach(directory => walk(path.join(src, directory)));
  const keys = new Set();
  for (const file of files) {
    const source = fs.readFileSync(file, 'utf8');
    const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, file.endsWith('tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS);
    function visit(node) {
      if (ts.isCallExpression(node) && node.expression.getText(ast) === 't' && node.arguments[0] && ts.isStringLiteralLike(node.arguments[0])) keys.add(node.arguments[0].text);
      ts.forEachChild(node, visit);
    }
    visit(ast);
  }
  const storyFile = path.join(src, 'components/public/HomeProductStory.tsx');
  const storySource = fs.readFileSync(storyFile, 'utf8');
  const storyAst = ts.createSourceFile(storyFile, storySource, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const translatedData = new Set(['stories', 'howSteps', 'shareWays', 'nfcSteps', 'qrSteps', 'useCases']);
  function visitStory(node) {
    if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && translatedData.has(node.name.text) && node.initializer) {
      function collect(value) {
        if (ts.isStringLiteralLike(value) && /[A-Za-z]{3}/.test(value.text) && !value.text.startsWith('/') && !['digital', 'nfc', 'qr', 'save', 'analytics'].includes(value.text)) keys.add(value.text);
        ts.forEachChild(value, collect);
      }
      collect(node.initializer);
    }
    ts.forEachChild(node, visitStory);
  }
  visitStory(storyAst);
  for (const [relative, names] of [
    ['components/dashboard/CardEditor.tsx', new Set(['steps', 'themes'])],
    ['app/templates/page.tsx', new Set(['categories'])],
    ['components/public/PublicHeader.tsx', new Set(['defaultNavigation'])],
    ['app/contact/page.tsx', new Set(['questions', 'aboutPoints'])],
  ]) {
    const filename = path.join(src, relative);
    const code = fs.readFileSync(filename, 'utf8');
    const ast = ts.createSourceFile(filename, code, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
    function collectData(node) {
      if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && names.has(node.name.text) && node.initializer) {
        function add(value) {
          if (ts.isStringLiteralLike(value) && /[A-Za-z]{2}/.test(value.text) && !value.text.startsWith('/') && !/^#[0-9a-f]{3,8}$/i.test(value.text)) keys.add(value.text);
          ts.forEachChild(value, add);
        }
        add(node.initializer);
      }
      ts.forEachChild(node, collectData);
    }
    collectData(ast);
  }

  const catalogs = ['customer-common-messages', 'customer-auth-messages', 'customer-card-messages', 'customer-workspace-messages', 'customer-chat-messages', 'customer-public-messages', 'customer-story-messages'];
  const arabic = { ...loadTS('lib/locale-catalogs.ts').adminTranslations.ar };
  for (const name of catalogs) {
    const value = loadTS(`lib/${name}.ts`);
    for (const dictionary of Object.values(value)) if (dictionary && typeof dictionary === 'object') Object.assign(arabic, dictionary);
  }
  const missing = [...keys].filter(key => !(key in arabic));
  assert.deepEqual(missing, [], `Missing Arabic entries for: ${missing.join(', ')}`);
});

test('the shared frontend locale boundary applies the selected document language and direction', () => {
  const source = fs.readFileSync(path.join(src, 'lib/customer-locale.tsx'), 'utf8');
  assert.match(source, /storageKey="frontend_language"/);
  assert.match(source, /dir=\{language === 'ar' \? 'rtl' : 'ltr'\}/);
  assert.match(source, /lang=\{language\}/);
});
