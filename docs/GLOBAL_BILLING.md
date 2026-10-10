# DBIZCARD global subscription billing

## Price and currency model
The existing website_pricing_plans.price, currency and billing_period remain the base price. Migration 000033 adds billing_config; no existing subscriptions or monetary snapshots are converted.

Global price configuration is accessed through the existing Plans admin screen's Global prices action. Each rule stores currency, optional ISO country code, billing_period, exact decimal amount, tax_bps, optional decimal_precision and rounding. Country-specific rules take priority over general currency rules. The base price applies only to its original currency and period. Missing prices are unavailable; there is no implicit FX conversion.

Tax is added to the subtotal and rounded half up to the ISO minor unit. Optional subtotal rounding uses reject (default), half_up, down or up at a precision no greater than the ISO exponent. All arithmetic on the server uses integers/big integers. The display calculator uses BigInt, with oversized unsafe browser amounts rejected. Both sides verify the displayed total at checkout.

### Example
```json
{
  "providers": ["razorpay"],
  "prices": [
    {"currency":"USD","country":"","billing_period":"monthly","amount":"10.00","tax_bps":0},
    {"currency":"USD","country":"US","billing_period":"monthly","amount":"9.99","tax_bps":750},
    {"currency":"USD","country":"","billing_period":"yearly","amount":"99.00","tax_bps":0},
    {"currency":"KWD","country":"KW","billing_period":"monthly","amount":"1.230","tax_bps":0,"decimal_precision":2,"rounding":"reject"}
  ]
}
```

## Registries
- Application: embedded SIX ISO 4217 List One, published 2026-09-17. Includes 165 unique codes with numeric minor-unit definitions (including fund units); metals and testing codes with N.A. precision are excluded. Existing currency names/symbols are preserved. New currency symbols are formatted through Intl on the frontend.
- Country suggestions: a current tender-currency snapshot from Unicode CLDR currencyData.json. It is a suggestion only. Multiple-currency territories use the first current tender currency in the registry.
- Razorpay: independent snapshot of its documented international currency table, plus INR. Published product support is not account enablement.
- Refresh the registries from their official sources, inspect additions/withdrawals and precision changes, then run the currency tests. Do not remove support needed for historical monetary snapshots without a compatibility migration.

Sources:
- https://www.six-group.com/dam/download/financial-information/data-center/iso-currrency/lists/list-one.xml
- https://github.com/unicode-org/cldr-json/blob/main/cldr-json/cldr-core/supplemental/currencyData.json
- https://github.com/razorpay/markdown-docs/blob/master/payments/international-payments.md
- https://github.com/razorpay/markdown-docs/blob/master/api/orders/create.md

## Merchant capabilities
Payment Settings retains the existing merchant currency selection/confirmation. The list now excludes activation of currencies outside Razorpay's published registry. A database confirmation is bound to test/live mode. Legacy unscoped confirmations are accepted only in test mode. The environment allowlist is still supported; bind it using RAZORPAY_CURRENCY_ALLOWLIST_MODE. An unset environment allowlist defaults to INR.

Global billing capabilities on the same Settings screen configure inactive application currencies and per-provider currencies, countries, minimum amounts in ISO minor units, method scopes and manual renewal. Capabilities are stored under billing_global_test or billing_global_live and require merchant confirmation. These declarations can further restrict checkout; they never expand the merchant currency allowlist or activate an adapter.

India/INR can declare merchant-enabled card, upi, netbanking, wallet, emi and paylater methods. International standard checkout can declare card support. Razorpay non-card method declarations require explicit IN/INR scope. Google Pay/PhonePe availability and card brands are resolved by Razorpay itself. Bank transfers and international local-method flows require separate approved integrations; they are not forced into this standard Orders checkout.

If methods are not configured, native Razorpay Checkout remains responsible for actual eligibility. The application does not advertise an assumed method list. Configured methods are intersected with country/currency restrictions, and native Checkout still enforces merchant eligibility. A configured minimum rejects an insufficient amount; it never increases the charge. Razorpay enforces any additional unpublished/account-specific restrictions during order creation.

Gateway enforces INR >= 100 paise and the final minor-unit digit = 0 for three-decimal currencies. A price/tax total that violates provider precision is rejected exactly; configure a chargeable price explicitly. ISO precision remains correct (JPY 0; KWD/BHD/OMR/JOD 3).

Stripe, PayPal and regional providers may implement the provider-neutral Gateway interface and declare capabilities. Only the existing Razorpay adapter is connected. Selecting a future provider in plan settings does not activate it or route payments.

## Checkout lifecycle and security
- Manual customer selection persists in localStorage and selected-plan URLs through login/registration; currency is sent explicitly to checkout with country, period and expected total.
- Country headers are ignored by default. BILLING_TRUST_LOCATION_HEADERS=true permits CF-IPCountry suggestions only behind a trusted proxy that strips client-supplied location headers.
- The server independently resolves the price and checks activation, provider permission, country/currency methods, minimums and exact minor units. A changed expected total returns HTTP 409 with the new quote for review, before creating a provider order.
- Compatible orders expire after 30 minutes. Reuse also fetches the provider order and checks its real ID, currency, amount and status.
- Before invalidating incompatible unpaid orders, the server checks their provider state. Paid/attempted orders require reconciliation and are not overwritten.
- Replacement covers changes in plan, amount, currency, country and billing period. Cancellation creates transaction history. Authorized orders are protected.
- Checkout signatures, signed callbacks, amount/currency and captured status are verified before activation. Existing unique event/payment IDs and transaction locks prevent duplicate fulfillment.
- A verified late capture after local cancellation is retained as paid with RECONCILIATION_REQUIRED and no subscription activation. Support must review refund/fulfillment. Retrying verification cannot falsely report subscription activation.
- Refunds, failure, pending and authorization retain existing handling. All paid/refunded snapshots remain unchanged.
- Monthly/yearly/lifetime purchases use the purchased period snapshot. Changing billing period on the same plan is allowed after payment; this implementation has manual renewals and no automatic mandates or prorating.

## Rollout
Apply backend/migrations/000033_global_subscription_prices.sql transactionally using the existing deployment migration procedure, before deploying the new API/frontend together. The migration is additive and preserves records; numeric widening may take table locks, so schedule it appropriately.

The implementation was tested on connection-local TEMP tables cloned from PostgreSQL public schema. The migration was applied only to those TEMP tables during tests. The public database and real merchant configuration were not changed. Stale real orders are guarded and invalidated on a subsequent checkout through the updated API; no batch cancellation of merchant orders was performed.

Use `go run ./cmd/billing-inspect` from backend for a read-only, non-secret configuration/state audit.

## Validation commands
- Backend: go test ./... -count=1 (set PAYMENT_TEST_DATABASE_URL to a migrated PostgreSQL database; payment tests use temporary tables and mocked providers).
- Frontend: node --test --test-reporter=spec
- TypeScript: node node_modules/typescript/bin/tsc --noEmit --incremental false
- Production: npm run build

Payment tests cover INR/USD/EUR/AED/SAR/QAR/KWD/BHD/OMR/JPY, unsupported/inactive currencies, exact precision, provider minimums, country/method restrictions, stale order replacement, changed quote consent, signature validation, duplicate callbacks, refunds and verified subscription activation.
