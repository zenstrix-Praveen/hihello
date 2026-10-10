# Subscription checkout operation

Run the backend with outbound HTTPS access to `api.razorpay.com:443`. A restricted
development sandbox returned checkout HTTP 502 despite valid test credentials.
The browser also needs access to `checkout.razorpay.com` and Razorpay's payment
resources. Do not substitute mock checkout for missing network access.

Apply `migrations/000032_subscription_trial_history.sql` before deploying this
backend. Preserve existing subscription and payment records.

The existing architecture uses Razorpay Orders and its embedded Checkout SDK,
not a hosted redirect URL or an automatic recurring mandate. Purchase activation
requires backend-verified capture or a signature-verified capture webhook.
Payments retain the purchased billing period. Renewal requires another payment.
Receipts use the existing payment and transaction history.

Trials use the existing subscription activation endpoint. Trial history persists
per customer, including cancelled trials and administrator edits. A trial takes
no payment or card authorization. The confirmation offers a free trial or explicit
consent to pay immediately. No automatic charge occurs when a trial ends; the
customer returns to checkout to purchase.

Configuration stays server-side: `PAYMENT_PROVIDER=razorpay`, `RAZORPAY_MODE`,
`RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET`, and `RAZORPAY_WEBHOOK_SECRET`.
`RAZORPAY_SUPPORTED_CURRENCIES` is an account availability allowlist, not a request
to enable currencies at the provider. Scope it with
`RAZORPAY_CURRENCY_ALLOWLIST_MODE=test` or `live` so a TEST list cannot enable LIVE
currencies. Without an explicit list, only INR is permitted. Alternatively, use
the existing mode-bound merchant settings in admin payment configuration.

All 12 currencies passed genuine Razorpay TEST order creation in this account:
USD, INR, AED, SAR, QAR, KWD, BHD, OMR, EGP, JOD, EUR, GBP. This does not establish
LIVE international-card acceptance or settlement availability. For LIVE operation,
provide intended LIVE credentials, confirm international payments and each allowed
currency with the live merchant, and configure a public HTTPS webhook URL ending
in `/api/v1/webhooks/razorpay`, with the same secret as the backend. Localhost cannot
receive remote provider webhooks.

Currency exponents come from `internal/currency`; conversion uses exact arithmetic.
KWD, BHD, OMR and JOD use three decimal places, but Razorpay requires the final
minor-unit digit to be zero. Unchargeable prices are rejected, never silently
rounded or converted.

Backend diagnostics record the provider stage, customer, plan, currency and period.
Provider errors are bounded and credential-redacted. The browser receives safe
messages for session, currency, configuration, network and provider failures.
