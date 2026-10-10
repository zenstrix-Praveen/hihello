const { test, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { chromium } = require('playwright-core');

const baseURL = process.env.NAV_BASE_URL || 'http://127.0.0.1:3000';
const origin = new URL(baseURL).origin;
const output = path.resolve('artifacts/navigation', new URL(baseURL).port || 'default');
let browser;
before(async () => {
  await fs.mkdir(output, { recursive: true });
  browser = await chromium.launch({ channel: process.env.NAV_BROWSER_CHANNEL || 'chrome', headless: true });
});
after(async () => { await browser?.close(); });

async function session(t, options = {}) {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, ...options });
  await context.tracing.start({ screenshots: true, snapshots: true, sources: true });
  const page = await context.newPage();
  page.setDefaultTimeout(30000);
  const events = [], errors = [], chunkFailures = [];
  page.on('pageerror', error => { errors.push(error.stack); events.push({ type: 'exception', text: error.stack }); });
  page.on('console', message => {
    if (message.type() !== 'error') return;
    events.push({ type: 'console', text: message.text() });
    if (/hydration|hydrating|Minified React error|ChunkLoadError/i.test(message.text())) errors.push(message.text());
  });
  page.on('framenavigated', frame => { if (frame === page.mainFrame()) events.push({ type: 'navigation', url: frame.url() }); });
  page.on('requestfailed', request => {
    events.push({ type: 'requestfailed', url: request.url(), failure: request.failure() });
    if (request.url().includes('/_next/static/') && request.failure()?.errorText !== 'net::ERR_ABORTED') chunkFailures.push(request.url());
  });
  page.on('response', response => {
    if (response.status() >= 300) events.push({ type: 'response', status: response.status(), url: response.url() });
    if (response.status() >= 400 && response.url().includes('/_next/static/')) chunkFailures.push(response.url());
  });
  const name = t.name.replace(/[^a-z0-9]+/gi, '-');
  t.after(async () => {
    // Hiding carets mutates input styles and can race hydration after a redirect.
    await page.screenshot({ path: path.join(output, `${name}.png`), caret: 'initial' });
    await fs.writeFile(path.join(output, `${name}.json`), JSON.stringify(events, null, 2));
    await context.tracing.stop({ path: path.join(output, `${name}.zip`) });
    await context.close();
    assert.deepEqual(errors, [], 'No browser runtime or hydration errors');
    assert.deepEqual(chunkFailures, [], 'No missing Next.js chunks');
  });
  return page;
}

const toggle = page => page.locator('[aria-controls="public-mobile-navigation"]');
const menu = page => page.locator('#public-mobile-navigation');
async function openMenu(page) {
  await toggle(page).click();
  await page.waitForFunction(() => document.querySelector('[aria-controls="public-mobile-navigation"]')?.getAttribute('aria-expanded') === 'true');
}
async function content(page, destination) {
  await page.waitForURL(url => url.pathname + url.hash === destination);
  assert.equal(new URL(page.url()).origin, origin, 'Navigation must preserve the LAN origin');
  if (!destination.startsWith('/admin')) {
    // The locale provider sets dir in an effect: don't stop at pre-hydration SSR.
    await page.waitForFunction(() => ['ltr', 'rtl'].includes(document.documentElement.dir));
  }
  const heading = page.locator('h1').first();
  await heading.waitFor({ state: 'visible' });
  const headingText = (await heading.innerText()).trim();
  assert.ok(headingText, 'Destination must render a real heading');
  assert.doesNotMatch(headingText, /^(404|500)$|not found|something went wrong|application error/i, 'An error page is not destination content');
  assert.equal(await page.locator('main[role="alert"]').count(), 0, 'An error boundary is not page content');
  if (destination === '/login' || destination === '/register' || destination === '/admin/login') {
    await page.locator('input[type="email"]').waitFor({ state: 'visible' });
    await page.locator('input[type="password"]').first().waitFor({ state: 'visible' });
  }
}
async function choose(page, name, destination) {
  await openMenu(page);
  await menu(page).getByRole('link', { name, exact: true }).click();
  await content(page, destination);
  if (await toggle(page).count()) assert.equal(await toggle(page).getAttribute('aria-expanded'), 'false');
}

async function chatBelowHeader(page) {
  await page.waitForFunction(() => {
    const panel = document.querySelector('[role="dialog"]');
    const header = document.querySelector('[data-public-header]');
    const navigation = document.querySelector('#public-mobile-navigation');
    return panel && header && navigation && parseFloat(getComputedStyle(navigation).maxHeight) === 0
      && Math.abs(panel.getBoundingClientRect().top - header.getBoundingClientRect().bottom) < 1;
  });
}

test('mobile clicks every public and authentication menu item with the real API', async t => {
  const page = await session(t);
  await page.goto(baseURL);
  // This marker survives client routing but not document navigation.
  await page.evaluate(() => { window.navigationTestMarker = 'same-document'; });
  for (const [name, destination] of [['Features', '/features'], ['Templates', '/templates'], ['Pricing', '/pricing'], ['Business', '/business'], ['Contact', '/contact'], ['Home', '/']]) {
    await choose(page, name, destination);
    assert.equal(await page.evaluate(() => window.navigationTestMarker), 'same-document', 'Use Next.js client navigation');
  }
  for (const [name, destination] of [['Login', '/login'], ['Get Started', '/register'], ['Login as Administrator', '/admin/login']]) {
    await choose(page, name, destination);
    await page.goBack();
    await content(page, '/');
  }
});

test('mobile Platform expands and every existing submenu link renders content', async t => {
  const page = await session(t);
  await page.goto(baseURL);
  await openMenu(page);
  await menu(page).getByRole('button', { name: 'Platform', exact: true }).click();
  assert.equal(new URL(page.url()).pathname, '/');
  const links = await page.locator('#platform-mobile-groups a').evaluateAll(nodes => nodes.map(node => ({ name: node.textContent, href: node.getAttribute('href') })));
  assert.equal(links.length, 20);
  for (const item of links) {
    if (!await page.locator('#platform-mobile-groups').count()) {
      await openMenu(page);
      await menu(page).getByRole('button', { name: 'Platform', exact: true }).click();
    }
    await page.locator('#platform-mobile-groups').getByRole('link', { name: item.name, exact: true }).click();
    await content(page, item.href);
    assert.equal(await toggle(page).getAttribute('aria-expanded'), 'false');
  }
});

async function cmsFixture(page, header) {
  // Only the CMS/API is controlled: Next, React, page components and clicks are real.
  await page.route('**/api/v1/**', async route => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith('/public/website/header')) return route.fulfill({ json: { ...header, admin_link_label: 'CMS ready' } });
    return route.fulfill({ status: 503, json: { error: 'API unavailable' } });
  });
}

test('CMS translated Platform entry is dropdown-only', async t => {
  const page = await session(t);
  await cmsFixture(page, { navigation_items: [{ label: 'Home', url: '/' }, { label: 'Features', url: '/features' }, { label: 'المنصة', url: '/platform/' }], platform_menu: { label: 'المنصة', enabled: true } });
  await page.goto(baseURL);
  await openMenu(page);
  await menu(page).getByRole('button', { name: 'المنصة', exact: true }).waitFor();
  assert.equal(await menu(page).getByRole('link', { name: 'المنصة', exact: true }).count(), 0, 'CMS labels must not introduce a nonexistent page');
  await menu(page).getByRole('button', { name: 'المنصة', exact: true }).click();
  await page.locator('#platform-mobile-groups').waitFor({ state: 'visible' });
  assert.equal(new URL(page.url()).pathname, '/');
});

test('CMS loopback links navigate on the current LAN origin', async t => {
  const page = await session(t);
  await cmsFixture(page, { navigation_items: [
    { label: 'Home', url: 'http://localhost:3000/' },
    { label: 'Features', url: 'http://localhost:3000/features' },
    { label: 'Templates', url: 'http://127.0.0.1:3000/templates' },
    { label: 'Pricing', url: 'http://localhost:3000/pricing' },
    { label: 'Business', url: 'http://localhost:3000/business' },
    { label: 'Contact', url: 'http://localhost:3000/contact' },
  ], login_button: { label: 'Login', url: 'http://localhost:3000/login' }, signup_button: { label: 'Get Started', url: 'http://localhost:3000/register' } });
  await page.goto(baseURL);
  await openMenu(page);
  await menu(page).getByRole('link', { name: 'CMS ready', exact: true }).waitFor();
  assert.equal(await menu(page).getByRole('link', { name: 'Features', exact: true }).getAttribute('href'), '/features');
  await toggle(page).click();
  for (const [name, destination] of [['Features', '/features'], ['Templates', '/templates'], ['Pricing', '/pricing'], ['Business', '/business'], ['Contact', '/contact'], ['Home', '/'], ['Login', '/login']]) await choose(page, name, destination);
  await page.goBack();
  await content(page, '/');
  await choose(page, 'Get Started', '/register');
});

test('mobile links keep their DOM identity when header scroll state changes', async t => {
  const page = await session(t);
  await page.goto(baseURL);
  await openMenu(page);
  const link = menu(page).getByRole('link', { name: 'Features', exact: true });
  await link.focus();
  await link.evaluate(element => { window.originalNavigationLink = element; });
  await page.evaluate(() => window.scrollTo(0, 120));
  await page.waitForFunction(() => document.querySelector('header')?.classList.contains('shadow-sm'));
  assert.equal(await link.evaluate(element => element === window.originalNavigationLink), true, 'A scroll render must not replace the link during a touch gesture');
  await link.click();
  await content(page, '/features');
});

test('CMS canonical-site links preserve query and hash while external links stay external', async t => {
  const page = await session(t);
  await cmsFixture(page, { navigation_items: [
    { label: 'Features', url: 'https://dbizcard.example/features?source=menu#digital-cards' },
    { label: 'External', url: 'https://external.example/features' },
  ] });
  await page.route('**/public/website/seo?*', route => route.fulfill({ json: { public_base_url: 'https://dbizcard.example' } }));
  await page.goto(baseURL);
  await openMenu(page);
  await page.waitForFunction(() => document.querySelector('#public-mobile-navigation a[href="/features?source=menu#digital-cards"]'));
  assert.equal(await menu(page).getByRole('link', { name: 'External', exact: true }).getAttribute('href'), 'https://external.example/features');
  await menu(page).getByRole('link', { name: 'Features', exact: true }).click();
  await content(page, '/features#digital-cards');
  assert.equal(new URL(page.url()).search, '?source=menu');
});

test('Arabic mobile routing keeps RTL and chat/menu interactions working', async t => {
  const page = await session(t);
  await cmsFixture(page, {});
  await page.goto(baseURL);
  await openMenu(page);
  await menu(page).locator('select').selectOption('ar');
  await page.waitForFunction(() => document.documentElement.dir === 'rtl');
  await page.locator('[data-customer-locale] > p[role="status"]').waitFor({ state: 'visible' });
  await toggle(page).click();
  for (const destination of ['/features', '/templates', '/pricing', '/business', '/contact', '/']) {
    await openMenu(page);
    await menu(page).locator(`a[href="${destination}"]`).first().click();
    await content(page, destination);
    assert.equal(await page.locator('[data-customer-locale]').getAttribute('dir'), 'rtl');
  }
  await page.locator('button.fixed[aria-expanded="false"]').click();
  await page.getByRole('dialog').waitFor({ state: 'visible' });
  await chatBelowHeader(page);
  await page.locator('#dbizcard-chat-input').fill('pricing');
  // Submit through the input; Next's development badge overlaps the RTL send button.
  await page.locator('#dbizcard-chat-input').press('Enter');
  await page.getByRole('dialog').getByText('pricing', { exact: true }).waitFor({ state: 'visible' });
  await openMenu(page);
  await page.getByRole('dialog').waitFor({ state: 'hidden' });
  await page.locator('button.fixed[aria-expanded="false"]').click();
  await page.getByRole('dialog').waitFor({ state: 'visible' });
  assert.equal(await toggle(page).getAttribute('aria-expanded'), 'false');
  await chatBelowHeader(page);
});

test('desktop dropdown, navigation and redirects remain functional', async t => {
  const page = await session(t, { viewport: { width: 1440, height: 900 }, isMobile: false, hasTouch: false });
  await page.goto(baseURL);
  await page.locator('[aria-controls="platform-mega-menu"]').hover();
  await page.locator('#platform-mega-menu').waitFor({ state: 'visible' });
  await page.keyboard.press('Escape');
  await page.locator('#platform-mega-menu').waitFor({ state: 'hidden' });
  await page.locator('header nav').first().getByRole('link', { name: 'Features', exact: true }).click();
  await content(page, '/features');
  for (const [oldPath, destination] of [['/about', '/contact#about-us'], ['/faq', '/contact#faq']]) {
    await page.goto(new URL(oldPath, baseURL).href);
    await content(page, destination);
  }
});
