package com.agenticpay.order.domain;

/** Thrown when a mutator is called on an order that is already COMPLETED. */
public class OrderCompletedException extends IllegalStateException {

  public OrderCompletedException(String orderId) {
    super("Order " + orderId + " is COMPLETED and immutable");
  }
}
