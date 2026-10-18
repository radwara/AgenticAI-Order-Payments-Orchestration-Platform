package com.platform.payment.provider;

/** Why a provider declined an operation. Declines are final: retrying will not change them. */
public enum DeclineReason {
  CARD_DECLINED,
  INSUFFICIENT_FUNDS,
  AMOUNT_EXCEEDS_AUTH,
  AMOUNT_EXCEEDS_CAPTURE,
  CURRENCY_MISMATCH,
  UNKNOWN_REFERENCE,
  INVALID_STATE
}
