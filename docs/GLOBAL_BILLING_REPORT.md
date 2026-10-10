# DBIZCARD global subscription payments — implementation report

Implementation and automated verification completed on 2026-10-09. The application still uses the existing subscription, payment and transaction modules. No real charge, provider activation, production deployment or public-database migration was performed.

## 1. INR/SAR mismatch and checkout state

The read-only local audit confirmed inconsistent checkout state:

- Professional's current database price is INR 100.000 per month.
- Three open local orders retain SAR amounts of 0.500, 1.500 and 100.000.
- The current source already contained some currency and minimum guards; there was no evidence of a current deliberate INR-to-SAR conversion.

These stale order snapshots explain the current currency conflict. The exact historical ₹0.50/SAR 1 display could not be reproduced from the present plan configuration, so its earlier conversion/rounding path cannot be established conclusively.

Checkout now independently resolves the selected price and expected total, verifies provider-order metadata before reuse, and replaces incompatible unpaid sessions only after checking provider state. Changes in plan, amount, currency, country or period create a fresh order. Old unpaid records retain cancellation transaction history. Authorized/paid/attempted provider orders require reconciliation.

The specific INR 0.50 regression is tested: a stale SAR order is invalidated, no replacement provider order is created, and the customer receives an INR minimum error. The application never substitutes SAR or increases INR 0.50 to another amount.

## 2. Application currencies

The embedded SIX ISO 4217 List One registry, dated 2026-09-17, contains 165 unique codes with numeric minor-unit definitions, including all requested currencies:

INR, USD, EUR, GBP, AED, SAR, QAR, KWD, BHD, OMR, JOD, EGP, CAD, AUD, SGD, MYR, JPY, CNY, CHF, NZD and ZAR.

The data also includes ISO fund units with four decimal places; metals/test codes with N.A. precision are excluded from chargeable currencies. Existing currency metadata is preserved. Admins can deactivate application currencies independently of provider acceptance.

Country suggestions use a 255-territory Unicode CLDR snapshot. Trusted location headers require explicit proxy configuration; a suggestion is applied only when the customer chooses it. Manual selection persists through pricing, authentication and checkout. English/Arabic controls, tax totals and currency-aware history formatting are included.

Application currency support does not imply payment acceptance.

## 3. Actual configured Razorpay acceptance

The inspected deployment is PAYMENT_PROVIDER=razorpay, RAZORPAY_MODE=test. Its environment allowlist is:

USD, INR, AED, SAR, QAR, KWD, BHD, OMR, EGP, JOD, EUR and GBP.

No merchant currency-confirmation or capability settings were present in the inspected database. Consequently, actual merchant-enabled test/live currencies and payment methods are **not independently verified**. The allowlist is configuration evidence only.

A separate embedded provider registry contains 129 codes from Razorpay's published currency table plus INR. Checkout intersects application support with configured merchant currency acceptance; admin settings cannot enable a code outside the published provider registry. Test-mode declarations cannot authorize live-mode currencies.

The current account has not been probed with real provider orders. All order/payment responses in automated tests are mocked.

## 4. Methods by country and provider

| Country/flow | Implemented handling | Merchant limitation |
|---|---|---|
| India, INR, Razorpay standard checkout | Configurable card, UPI, netbanking, wallet, EMI and Pay Later scopes | Each method must be merchant-confirmed; native checkout still decides eligibility |
| Google Pay/PhonePe | Resolved within eligible native UPI checkout | No forced app availability |
| International Razorpay standard checkout | Merchant-enabled card checkout, restricted by configured country/currency | Visa/Mastercard/Amex, international card access and issuer eligibility are not assumed |
| International bank transfers, wallets and local methods | No unsupported options forced into standard Orders checkout | Separate provider/product onboarding and approved integration may be needed |
| Stripe/PayPal/future providers | Capability declarations and provider-neutral Gateway extension point | No new adapters activated, no routing or charging through them |

When no explicit method list is saved, native Razorpay Checkout determines actual methods. This application does not advertise a fabricated country-wide method list.

## 5. Database and code changes

Migration: backend/migrations/000033_global_subscription_prices.sql.

It adds billing_config to existing plans, checkout country/quote snapshots to existing payments, an open-checkout index, and monetary precision widening to NUMERIC(18,4). Existing base prices, subscriptions and transaction values are preserved.

Main changes:
- Currency registry: backend/internal/currency/catalog.go and iso4217.xml.
- Exact price/tax/rounding resolution: backend/internal/pricing.
- Provider interface, capabilities, stale-order guards, verification and late-capture reconciliation: backend/internal/payments.
- Provider-order retrieval: backend/internal/razorpay/client.go.
- Existing website plan response exposes explicit price variants.
- Admin Plans → Global prices: country/currency/monthly/yearly rules, tax, precision, rounding and allowed providers.
- Admin Payment Settings: currency activation, provider countries/currencies, minimums and method scopes.
- Pricing/billing/authentication frontend preserves selected currency/country and reviewed total.
- Read-only audit command: backend/cmd/billing-inspect.
- Configuration/rollout documentation: docs/GLOBAL_BILLING.md and DEPLOYMENT.md.

Recurring charges were not introduced. Monthly/yearly plans use manual renewal, and same-plan period changes take effect only after verified payment. Prorating is not implemented.

Verified captures after cancellation are retained as paid with RECONCILIATION_REQUIRED, without activating an obsolete subscription. They remain visible for manual refund/fulfillment review.

## 6. Test results

| Check | Result |
|---|---|
| go test ./... -count=1 | Passed |
| PostgreSQL payment integration tests | Passed with PAYMENT_TEST_DATABASE_URL enabled; connection-local TEMP tables and mocked Razorpay |
| TypeScript --noEmit --incremental false | Passed |
| Frontend node --test | 138 passed, 0 failed |
| Next.js production build | Passed; 61 pages generated |

Coverage includes INR/USD/EUR/AED/SAR/QAR/KWD/BHD/OMR/JPY order creation and verified activation, unsupported/inactive currencies, zero/three-decimal rules, configured minimums, country/method restrictions, exact taxes, rounding, quote-change consent, stale replacement, provider-paid protection, signature failures, duplicate processing, refunds, late captures, admin price persistence and same-plan billing-period changes.

Unrelated optional database suites that require their own disposable-database variables remain skipped. Production builds emit the pre-existing warning about a package-lock.json outside the repository; builds complete successfully.

## 7. Remaining merchant and rollout limitations

- Apply migration 000033 before deploying the updated API/frontend together. It was applied only to TEMP test tables, not to the public database.
- The three real local SAR orders were inspected, not batch-cancelled. The updated checkout checks their real provider state before invalidating them; remotely paid/attempted orders require reconciliation.
- Confirm actual currencies, international-card approval, enabled methods and non-INR minimums in the merchant account for each payment mode.
- INR minimum enforcement is 100 paise. KWD/BHD/OMR/JOD use ISO three-decimal precision, but Razorpay requires the last minor-unit digit to be zero. Non-chargeable exact totals are rejected, never silently rounded by checkout.
- Configure country-specific tax rates appropriate to the business. This change does not introduce an automated tax-registration or tax-jurisdiction service.
- Saved capability declarations constrain native checkout; they do not enable gateway features or guarantee issuer/payment eligibility.
- No automatic subscriptions, new-provider onboarding or international transfer/local-method integration was enabled.

## 8. Recommended additional providers

Evaluate Stripe for supported merchant jurisdictions requiring broader currency/local-method and recurring billing options; validate account country availability and currency-specific limits first.

Evaluate PayPal for markets where customers prefer PayPal wallets and the merchant can receive the required currencies.

Evaluate Checkout.com or an approved regional provider for Gulf local methods such as mada and installments such as Tamara. Provider/product/account approval is required before integrating or routing payments.

Sources:
- [SIX currency standards](https://www.six-group.com/en/products-services/financial-information/data-standards.html)
- [Unicode CLDR currency data](https://github.com/unicode-org/cldr-json/blob/main/cldr-json/cldr-core/supplemental/currencyData.json)
- [Razorpay international payment documentation](https://github.com/razorpay/markdown-docs/blob/master/payments/international-payments.md)
- [Razorpay order/precision documentation](https://github.com/razorpay/markdown-docs/blob/master/api/orders/create.md)
- [Stripe supported currencies](https://docs.stripe.com/currencies)
- [PayPal countries and currencies](https://www.paypal.com/webapps/mpp/country-worldwide)
- [Checkout.com payment methods](https://www.checkout.com/payment-methods)
