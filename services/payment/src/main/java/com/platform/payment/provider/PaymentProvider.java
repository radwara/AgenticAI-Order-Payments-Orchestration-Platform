package com.platform.payment.provider;

/**
 * Pluggable payment provider (PAY-2). Handles opaque provider tokens only; raw card data never
 * enters this interface. Implementations never throw for bad input or expected failures: they
 * return a {@link ProviderResult} (null or non-positive arguments are Declined INVALID_ARGUMENT).
 *
 * <h2>Idempotency expectations for implementers</h2>
 *
 * <ul>
 *   <li>Every operation takes an {@code idempotencyKey}; a null or blank key is rejected as
 *       INVALID_ARGUMENT. Calling again with the same key and the same arguments must return the
 *       same result and must not apply the effect twice (no double capture, no double refund).
 *   <li>Reusing a key with different arguments must be rejected as IDEMPOTENCY_KEY_REUSED, never
 *       applied as a new operation and never answered with the first result.
 *   <li>A {@link ProviderResult.TransientError} means the outcome is unknown; callers retry with
 *       the same key, and the retry must apply the effect at most once.
 *   <li>A {@link ProviderResult.Declined} is final for that key.
 *   <li>Total captured must never exceed the authorized amount, and total refunded must never
 *       exceed the captured amount, even for values near {@code Long.MAX_VALUE}.
 * </ul>
 */
public interface PaymentProvider {

  /**
   * Reserves funds for a payment method.
   *
   * @param token opaque provider token for the payment method
   * @param amount amount to authorize, greater than zero
   * @param idempotencyKey caller-generated key, see the class documentation
   * @return {@link ProviderResult.Success} carrying the authorization reference
   */
  ProviderResult authorize(String token, Money amount, String idempotencyKey);

  /**
   * Captures funds from an authorization, in full or in part; may be called more than once while
   * the cumulative total stays within the authorization.
   *
   * @return Declined with {@link DeclineReason#AMOUNT_EXCEEDS_AUTH} if the total would exceed it
   */
  ProviderResult capture(String token, String providerRef, Money amount, String idempotencyKey);

  /** Releases an authorization that has not been captured. */
  ProviderResult voidAuthorization(String token, String providerRef, String idempotencyKey);

  /**
   * Refunds captured funds, in full or in part.
   *
   * @return Declined with {@link DeclineReason#AMOUNT_EXCEEDS_CAPTURE} if total refunds would
   *     exceed the captured amount
   */
  ProviderResult refund(String token, String providerRef, Money amount, String idempotencyKey);
}
