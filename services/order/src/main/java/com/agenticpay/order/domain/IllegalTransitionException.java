package com.agenticpay.order.domain;

/** Thrown when a requested order status change is not in the transition table. */
public class IllegalTransitionException extends RuntimeException {

  private final OrderStatus from;
  private final OrderStatus to;

  public IllegalTransitionException(OrderStatus from, OrderStatus to) {
    super("Illegal order transition: " + from + " -> " + to);
    this.from = from;
    this.to = to;
  }

  public OrderStatus from() {
    return from;
  }

  public OrderStatus to() {
    return to;
  }
}
