# Customer subscription upgrade repair

> Implementation is authorized by the user's eight-phase repair request. Execute inline and preserve existing pricing card design.

**Goal:** Resume the selected customer plan through login, confirmation, checkout, and verified activation using the existing billing system.

**Architecture:** Keep `/dashboard/billing` as the confirmation and subscription page. Preserve validated plan/billing query parameters through customer login and expired-session redirects. Continue using authenticated Razorpay order creation and verified capture/webhooks; never activate a paid plan from a return URL or trial setting.

**Tech stack:** Next.js 16 / React 19 / TypeScript, Go, PostgreSQL, Razorpay.

**Spec:** User's DBIZCARD eight-phase repair request in this conversation.

## Constraints and review focus

- Preserve pricing card styling and free registration.
- Invalid or expired tokens must retain upgrade intent; arbitrary external next URLs must be rejected.
- Selected plans must open confirmation once, including annual plans, unavailable currencies, and missing/edited plans.
- Paid plans with trial settings must still use checkout for an Upgrade Plan purchase.
- Double clicks, provider errors, cancellation, pending verification, and failed capture must produce feedback without duplicate activation.
- Real merchant capability and sandbox capture must be reported separately from isolated test fixtures.

## Tasks

1. Add failing component regression tests for pricing clicks, login resumption, selected-plan confirmation, paid trial checkout, unsupported currency feedback, and duplicate submissions. Run `node --test tests/subscription-upgrade.test.cjs`.
2. Repair pricing navigation and customer login/session continuation. Add a narrow safe destination helper; keep Free Get Started registration.
3. Repair the existing Billing confirmation/checkout actions, selected-period validation, loading/error feedback, and paid trial routing. Validate the plan period on the backend against the selected plan's authoritative period.
4. Run isolated PostgreSQL payment integration tests for trusted amount/currency, verified activation, ownership, failed/cancelled payment, idempotency, webhook activation, and billing history. Extend tests where the repair changes behavior.
5. Run `npx tsc --noEmit`, `npm test`, `npm run build`, `go test ./...`; browser regression checks at localhost:3000 for desktop/mobile English/Arabic, actual public API connectivity, login redirect, confirmation, errors, and checkout.
6. Review modified files and write a report distinguishing verified running behavior, isolated fixture tests, and any merchant/sandbox configuration blockers.

## Progress

- Investigation: the visible button had an action, but Billing never opened the requested plan confirmation. Paid logged-out clicks went to registration, and login/expired-session redirects lost the selection. CMS CTA URLs could also bypass the action (current paid plans have empty CTA URLs). Paid trial flags bypassed checkout, and disabled currency actions prevented feedback.
- Runtime: source Go API started on 8080; health verifies PostgreSQL/Redis. Pricing API returns Professional id 10 QAR 25, Business id 14 SAR 100, Normal Users id 31 free.
- Ruling: repair in supplied workspace because all project files are untracked in the outer repository; a git worktree would omit the project. No commit or merge is requested.
- Implementation: preserved safe customer login destinations, opened existing confirmation, added loading/duplicate/error feedback and paid-trial checkout; preserved free registration and pricing card styles.
- Payment integrity: fixed transaction/pool lookup deadlock; snapshotted purchased billing period; replaced prior trial subscriptions only after verified capture; retained cancellation through delayed webhooks and verification races. Stored order price/currency changes require fresh visible confirmation.
- Verification: TypeScript, all 131 frontend tests, production build, and Go suite with isolated PostgreSQL payment tests pass. All 14 running browser tests pass, including real customer login/free activation/unsupported checkout in English and Arabic, and mobile/desktop pricing actions. Independent review has no remaining important findings. The report and final passing log are in web/artifacts/upgrade/report.txt and browser-verified.log.
- Merchant limit: test credentials authenticate successfully, but runtime currency configuration is INR-only and merchant confirmation is false. QAR/SAR real sandbox checkout/capture cannot be verified without merchant activation and the existing currency settings configuration. No paid production/local customer subscription was activated as a substitute.
