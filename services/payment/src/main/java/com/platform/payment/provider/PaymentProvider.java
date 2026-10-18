package com.platform.payment.provider;

/**
 * Pluggable payment provider (PAY-2). Handles opaque provider tokens only; raw card data never
 * enters this interface.
 *
 * <h2>Idempotency expectations for implementers</h2>
 *
 * <ul>
 *   <li>Every operation takes an {@code idempotencyKey}. Calling an operation again with the same
 *       key and the same arguments must return the same result and must not apply the effect twice
 *       (no double capture, no double refund).
 *   <li>Reusing a key with different arguments is a caller bug; implementations must not apply it
 *       as a new operation.
 *   <li>A {@link ProviderResult.TransientError} leaves the outcome unknown; callers retry with the
 *       same key, so implementations must deduplicate on it.
 *   <li>A {@link ProviderResult.Declined} is final for that key.
 *   <li>Implementations must never throw for expected failures; they return a result.
 *   <li>Total captured must never exceed the authorized amount, and total refunded must never
 *       exceed the captured amount.
 * </ul>
 */
public interface PaymentProvider {

  /**
   * Reserves funds for a payment method.
   *
   * @param token opaque provider token for the payment method
   * @param amount amount to authorize
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
