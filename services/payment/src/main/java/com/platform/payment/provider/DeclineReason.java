package com.platform.payment.provider;

/** Why a provider declined an operation. Declines are final: retrying will not change them. */
public enum DeclineReason {
  /** The payment method was refused. */
  CARD_DECLINED,
  /** The payment method lacks funds for the amount. */
  INSUFFICIENT_FUNDS,
  /** Capture would make the captured total exceed the authorized amount. */
  AMOUNT_EXCEEDS_AUTH,
  /** Refund would make the refunded total exceed the captured amount. */
  AMOUNT_EXCEEDS_CAPTURE,
  /** The amount's currency differs from the authorization's. */
  CURRENCY_MISMATCH,
  /** No authorization with that reference exists for the token. */
  UNKNOWN_REFERENCE,
  /** The authorization's state does not allow the operation (for example void after capture). */
  INVALID_STATE,
  /** A null, blank or non-positive argument. */
  INVALID_ARGUMENT,
  /** The idempotency key was already used with different arguments. */
  IDEMPOTENCY_KEY_REUSED
}
