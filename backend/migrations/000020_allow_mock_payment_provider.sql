-- Permit local-only demo payments in the existing payment ledger.
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_provider_check;
ALTER TABLE payments
    ADD CONSTRAINT payments_provider_check
    CHECK (provider IN ('razorpay', 'mock'));
