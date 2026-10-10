const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const React = require('react');

const root = path.join(__dirname, '..');

function load(file, mocks) {
  const source = fs.readFileSync(path.join(root, file), 'utf8');
  const output = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: false },
  }).outputText;
  const mod = { exports: {} };
  new Function('require', 'module', 'exports', output)((id) => id in mocks ? mocks[id] : require(id), mod, mod.exports);
  return mod.exports;
}

function hooks() {
  const states = [], refs = [], effects = [];
  let cursor = 0;
  const react = {
    ...React,
    useState(initial) {
      const index = cursor++;
      if (!(index in states)) states[index] = typeof initial === 'function' ? initial() : initial;
      return [states[index], update => { states[index] = typeof update === 'function' ? update(states[index]) : update; }];
    },
    useRef(initial) {
      const index = cursor++;
      if (!(index in refs)) refs[index] = { current: initial };
      return refs[index];
    },
    useEffect(effect) { effects.push(effect); },
  };
  return { react, effects, render(component, props = {}) { cursor = 0; effects.length = 0; return component(props); } };
}

function find(node, predicate) {
  if (!node || typeof node !== 'object') return undefined;
  if (predicate(node)) return node;
  for (const child of [node.props?.children].flat(Infinity)) {
    const found = find(child, predicate);
    if (found) return found;
  }
}

const chatMocks = (react, apiRequest = async () => ({})) => ({
  react,
  'next/link': { default: 'a' },
  'lucide-react': { Bot: 'Bot', ChevronDown: 'ChevronDown', MessageCircle: 'MessageCircle', Minus: 'Minus', Send: 'Send', Sparkles: 'Sparkles', Trash2: 'Trash2', X: 'X' },
  '@/lib/customer-locale': { useCustomerLocale: () => ({ t: value => value, error: error => `API error: ${error.message}` }) },
  '@/lib/user-api': { apiRequest },
  '@/lib/chatbot-knowledge': load('src/lib/chatbot-knowledge.ts', {}),
});

test('hamburger click opens and closes the React menu, and a route click closes it', t => {
  const originalWindow = global.window;
  global.window = Object.assign(new EventTarget(), { location: { origin: 'http://192.168.29.31:3000' } });
  t.after(() => { if (originalWindow === undefined) delete global.window; else global.window = originalWindow; });
  const runtime = hooks();
  const module = load('src/components/public/PublicHeader.tsx', {
    react: runtime.react,
    'next/link': { default: 'a' },
    'next/navigation': { usePathname: () => '/' },
    'lucide-react': { Menu: 'Menu', X: 'X', LogIn: 'LogIn', ArrowRight: 'ArrowRight', ChevronDown: 'ChevronDown' },
    '@/components/public/Chatbot': { ChatbotLauncher: () => null },
    '@/components/public/useWebsiteSection': { useWebsiteSection: (_section, defaults) => defaults },
    '@/components/brand/BrandLogo': { default: () => null },
    '@/lib/platform-menu': { platformMenuGroups: [] },
    '@/lib/public-navigation': load('src/lib/public-navigation.ts', {}),
    '@/lib/customer-locale': { useCustomerLocale: () => ({ t: value => value }) },
    '@/components/locale/CustomerLanguageSwitcher': { default: () => null },
  });
  const Header = module.default;
  let tree = runtime.render(Header);
  runtime.effects[2]();
  const chatRuntime = hooks();
  const Chat = load('src/components/public/Chatbot.tsx', chatMocks(chatRuntime.react)).ChatbotLauncher;
  let chatTree = chatRuntime.render(Chat, { audience: 'public' });
  chatRuntime.effects[1]();
  let toggle = find(tree, node => node.type === 'button' && node.props?.['aria-controls'] === 'public-mobile-navigation');
  assert.equal(toggle.props['aria-expanded'], false);
  assert.match(toggle.props.className, /touch-manipulation/, 'hamburger is configured as a direct touch target');
  assert.match(toggle.props.className, /z-\[70\]/, 'hamburger is above the public header content');
  toggle.props.onClick();
  tree = runtime.render(Header);
  toggle = find(tree, node => node.type === 'button' && node.props?.['aria-controls'] === 'public-mobile-navigation');
  assert.equal(toggle.props['aria-expanded'], true);
  chatTree = chatRuntime.render(Chat, { audience: 'public' });
  find(chatTree, node => node.type === 'button' && node.props?.['aria-label'] === 'Open support chat').props.onClick();
  tree = runtime.render(Header);
  toggle = find(tree, node => node.type === 'button' && node.props?.['aria-controls'] === 'public-mobile-navigation');
  assert.equal(toggle.props['aria-expanded'], false, 'opening chat closes navigation');
  chatTree = chatRuntime.render(Chat, { audience: 'public' });
  assert.ok(find(chatTree, node => node.type === 'section' && node.props?.role === 'dialog'));
  toggle.props.onClick();
  chatTree = chatRuntime.render(Chat, { audience: 'public' });
  assert.equal(find(chatTree, node => node.type === 'section' && node.props?.role === 'dialog'), undefined, 'opening navigation closes chat');
  tree = runtime.render(Header);
  toggle = find(tree, node => node.type === 'button' && node.props?.['aria-controls'] === 'public-mobile-navigation');
  assert.equal(toggle.props['aria-expanded'], true);
  const featuresLink = find(tree, node => node.props?.href === '/features' && typeof node.props?.onClick === 'function');
  assert.ok(featuresLink, 'mobile Features link has a click handler');
  featuresLink.props.onClick();
  tree = runtime.render(Header);
  toggle = find(tree, node => node.type === 'button' && node.props?.['aria-controls'] === 'public-mobile-navigation');
  assert.equal(toggle.props['aria-expanded'], false);
});

test('chat launcher opens, sends a message, closes, and reopens through its React handlers', t => {
  const originalWindow = global.window;
  global.window = { dispatchEvent() {}, addEventListener() {}, removeEventListener() {}, setTimeout() {} };
  t.after(() => { if (originalWindow === undefined) delete global.window; else global.window = originalWindow; });
  const runtime = hooks();
  const Chat = load('src/components/public/Chatbot.tsx', chatMocks(runtime.react)).ChatbotLauncher;
  let tree = runtime.render(Chat, { audience: 'public' });
  let launcher = find(tree, node => node.type === 'button' && node.props?.['aria-label'] === 'Open support chat');
  assert.ok(launcher);
  assert.match(launcher.props.className, /touch-manipulation/, 'chat launcher is configured as a direct touch target');
  assert.match(launcher.props.className, /z-\[91\]/, 'chat launcher is above regular page content');
  launcher.props.onClick();
  tree = runtime.render(Chat, { audience: 'public' });
  assert.ok(find(tree, node => node.type === 'section' && node.props?.role === 'dialog'));
  const input = find(tree, node => node.type === 'input' && node.props?.id === 'dbizcard-chat-input');
  input.props.onChange({ target: { value: 'pricing' } });
  tree = runtime.render(Chat, { audience: 'public' });
  const form = find(tree, node => node.type === 'form');
  form.props.onSubmit({ preventDefault() {} });
  tree = runtime.render(Chat, { audience: 'public' });
  assert.ok(find(tree, node => node.props?.message?.text.includes('Pricing page lists the plans currently configured')));
  const close = find(tree, node => node.type === 'button' && node.props?.['aria-label'] === 'Close chat');
  close.props.onClick();
  tree = runtime.render(Chat, { audience: 'public' });
  assert.equal(find(tree, node => node.type === 'section' && node.props?.role === 'dialog'), undefined);
  find(tree, node => node.type === 'button' && node.props?.['aria-label'] === 'Open support chat').props.onClick();
  tree = runtime.render(Chat, { audience: 'public' });
  assert.ok(find(tree, node => node.type === 'section' && node.props?.role === 'dialog'));
});

test('customer chat surfaces account API errors while leaving support available', async t => {
  const originalWindow = global.window;
  global.window = { dispatchEvent() {}, addEventListener() {}, removeEventListener() {}, setTimeout() {} };
  t.after(() => { if (originalWindow === undefined) delete global.window; else global.window = originalWindow; });
  const runtime = hooks();
  const Chat = load('src/components/public/Chatbot.tsx', chatMocks(runtime.react, async () => { throw new Error('offline'); })).ChatbotLauncher;
  let tree = runtime.render(Chat, { audience: 'customer' });
  find(tree, node => node.type === 'button' && node.props?.['aria-label'] === 'Open support chat').props.onClick();
  tree = runtime.render(Chat, { audience: 'customer' });
  for (const effect of runtime.effects) effect();
  await new Promise(setImmediate);
  tree = runtime.render(Chat, { audience: 'customer' });
  assert.ok(find(tree, node => node.props?.role === 'status' && String(node.props?.children).includes('API error: offline')));
  assert.ok(find(tree, node => node.type === 'form'), 'chat input remains available after account API failure');
});
