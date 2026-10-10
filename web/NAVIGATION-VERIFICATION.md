# Mobile navigation investigation — 2026-10-08

## Findings and changes

The App Router contains `/`, `/features`, `/templates`, `/pricing`, `/business`, `/contact`, `/login`, `/register`, and `/admin/login`. There is no `/platform` page. `platformMenuGroups` defines 20 links into existing Features, Templates, Pricing and Business pages. The CMS schema permits only a label and enabled flag for `platform_menu`, not standalone page content. Platform is therefore dropdown-only; no new page or content was invented.

Two defects were reproduced with browser-controlled CMS responses before the fixes:

1. `PublicHeader` removed a CMS Platform entry by its English label only. A translated label with `/platform/` left an extra link to the nonexistent route. Platform is now recognized by route, including trailing slashes, and rendered only as the existing dropdown. Insertion after Features also uses the route instead of the English label.
2. Absolute CMS frontend links such as `http://localhost:3000/features` remained external anchors. On a phone, localhost addresses the phone, not the LAN server. Recognized loopback frontend links, current-origin links and links to the configured SEO public origin now become relative Next.js links. Paths, queries and hashes are retained; unrelated external links remain external. This applies to login/signup actions too.

`RenderLink` was also defined inside `PublicHeader`, creating a new component type on every header render. It is now declared at module scope so menu/scroll updates retain the same link DOM nodes. A browser test checks node identity across a scroll update and then clicks the link.

A later Arabic browser run reproduced a separate obstruction: the translation-fallback notice pushed the public header below its usual position, while the chatbot's hardcoded 80px top edge covered the hamburger. The public chat panel now measures the header's bottom edge and updates on resize/scroll/layout changes. The regression deliberately shows the Arabic fallback notice, verifies the panel does not overlap the header, sends a chat message, and switches between chat and navigation.

These are confirmed code defects, **not proof of the cause of the reported white pages**. Before editing, clicking all seven valid routes on the live LAN development server rendered their headings successfully. No React runtime/hydration exceptions or missing Next static chunks occurred. The general HTTP-200/white-page symptom could not be reproduced in this checkout.

## Runtime investigation

- Tested Chrome with a 390 × 844 mobile viewport, touch enabled, against `http://192.168.29.31:3000`, and the production build at port 3100. Desktop uses 1440 × 900.
- Recorded actual clicks, URL transitions, console errors, uncaught page exceptions, failed requests and HTTP errors, plus Playwright traces/screenshots. A window marker verifies that public menu transitions stay in the same document through Next client routing.
- The backend on LAN port 8080 refused connections during the initial live run. Existing public CMS fallbacks still rendered page content. No fallback or publication rules were changed. Explicit CMS `enabled: false` remains respected.
- Existing `/about` and `/faq` redirects reach `/contact#about-us` and `/contact#faq`, preserving origin. Public navigation showed no unexpected auth redirects. Login/register/admin login forms rendered; real authenticated API flows could not be exercised with the backend unavailable.
- No service-worker registration was found in application source/public scripts. Fresh and reused browser contexts showed no failed static chunks or hydration errors. This does not establish the state of a previously opened physical-phone tab or an older deployment's cached assets.

## Verification

- `npx tsc --noEmit`: passed.
- `npm test`: 107 passed, 0 failed.
- `npm run build`: passed, 61 pages generated.
- `npm run test:browser`: eight browser scenarios, covering every top-level mobile destination, all 20 submenu links, translated CMS Platform, CMS loopback/canonical links, query/hash preservation, link identity, Arabic RTL, chatbot/menu coordination, desktop navigation and redirects. Development run: 8 passed, 0 failed. Production run: 8 passed, 0 failed.
- The test harness waits for locale hydration and captures screenshots with the caret unchanged. Playwright's default caret-hiding screenshot mutation otherwise raced hydration after a redirect. Chat submission uses Enter because Next's development badge overlaps the RTL send button; that badge does not exist in production.

## Running the browser tests

Install dependencies with `npm ci`. Chrome must be installed; set `NAV_BROWSER_CHANNEL=msedge` to use installed Edge instead. Start the app, then run in another PowerShell window:

```powershell
$env:NAV_BASE_URL = 'http://192.168.29.31:3000'
npm run test:browser
```

The tests do not start or stop your server or require database mutations. CMS regression scenarios intercept only API responses; actual Next/React code, DOM clicks and routing run in the browser. The live scenarios use the configured API. Artifacts are written under `artifacts/navigation/<port>/` and ignored by Git. Browser checks are separate from `npm test` because they require a running server and a browser installation.

## Physical-phone verification still needed

On the affected phone/browser, open the same LAN address and click every menu item in English and Arabic, including Platform subitems and authentication links. Check both a fresh tab and the originally affected tab. Confirm the address retains the LAN hostname/port, page content appears, and menu/chat touch gestures behave correctly. If the white page persists, capture its exact URL, whether the header remains, console errors and failed JS/RSC/API requests using remote debugging. That evidence is needed to identify the still-unreproduced white-page cause.

For production, configure a browser-reachable `NEXT_PUBLIC_API_URL` as described in the deployment guide. Development's LAN API-host substitution does not apply to a production build.

## Files changed

- `src/components/public/PublicHeader.tsx`
- `src/components/public/Chatbot.tsx`
- `src/lib/public-navigation.ts` (new)
- `browser/navigation.cjs` (new)
- `tests/mobile-interactions.test.cjs` (updated module/window mocks)
- `package.json`, `package-lock.json` (browser test script and Playwright Core)
- `../.gitignore` (generated browser artifacts)
- `NAVIGATION-VERIFICATION.md` (this report)

Next also regenerates its ignored build output, TypeScript cache and generated `next-env.d.ts` during verification.
