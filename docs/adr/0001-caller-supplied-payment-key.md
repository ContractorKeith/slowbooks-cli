# Payments require a caller-supplied key

SlowBooks Pro deduplicates payment writes by `Idempotency-Key`. A random key per run would let a re-run after a timeout book the payment twice, and a key hashed from the body would silently merge two genuine identical payments. So `payments record` and `bill-payments record` require `--key` (for example `chk-1042`), and a missing key is a usage error.
