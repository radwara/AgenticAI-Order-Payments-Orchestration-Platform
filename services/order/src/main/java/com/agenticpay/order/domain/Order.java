package com.agenticpay.order.domain;

import java.util.Objects;

/**
 * Order aggregate. Status changes go only through {@link OrderStateMachine}. Once COMPLETED the
 * order is immutable: every mutator throws {@link OrderCompletedException}.
 */
public final class Order {

  private final String id;
  private final OrderStateMachine stateMachine;
  private OrderStatus status = OrderStatus.CREATED;
  private long totalMinorUnits;

  public Order(String id, OrderStateMachine stateMachine) {
    this.id = Objects.requireNonNull(id, "id");
    this.stateMachine = Objects.requireNonNull(stateMachine, "stateMachine");
  }

  public String id() {
    return id;
  }

  public OrderStatus status() {
    return status;
  }

  /** Order total in integer minor units (never floating point). */
  public long totalMinorUnits() {
    return totalMinorUnits;
  }

  /**
   * Moves the order to {@code target}.
   *
   * @throws OrderCompletedException if the order is COMPLETED
   * @throws IllegalTransitionException if the transition is not legal
   */
  public void transitionTo(OrderStatus target) {
    assertMutable();
    status = stateMachine.transition(status, target);
  }

  /** Cancels the order; legal only before fulfilment starts. */
  public void cancel() {
    transitionTo(OrderStatus.CANCELLED);
  }

  /**
   * Sets the total in minor units.
   *
   * @throws OrderCompletedException if the order is COMPLETED
   */
  public void updateTotal(long minorUnits) {
    assertMutable();
    this.totalMinorUnits = minorUnits;
  }

  private void assertMutable() {
    if (status == OrderStatus.COMPLETED) {
      throw new OrderCompletedException(id);
    }
  }
}
