package com.agenticpay.order.domain;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;

class OrderTest {

  private static Order orderIn(OrderStatus target) {
    Order order = new Order("ord-1", new OrderStateMachine());
    OrderStatus[] path = {
      OrderStatus.PRICED,
      OrderStatus.RESERVED,
      OrderStatus.PAID,
      OrderStatus.FULFILLING,
      OrderStatus.COMPLETED
    };
    for (OrderStatus step : path) {
      if (order.status() == target) {
        break;
      }
      order.transitionTo(step);
    }
    return order;
  }

  @Test
  void new_order_starts_in_created() {
    Order order = new Order("ord-1", new OrderStateMachine());
    assertThat(order.id()).isEqualTo("ord-1");
    assertThat(order.status()).isEqualTo(OrderStatus.CREATED);
  }

  @Test
  void follows_the_happy_path_to_completed() {
    assertThat(orderIn(OrderStatus.COMPLETED).status()).isEqualTo(OrderStatus.COMPLETED);
  }

  @Test
  void illegal_transition_throws_and_leaves_status_unchanged() {
    Order order = new Order("ord-1", new OrderStateMachine());
    assertThatThrownBy(() -> order.transitionTo(OrderStatus.PAID))
        .isInstanceOf(IllegalTransitionException.class);
    assertThat(order.status()).isEqualTo(OrderStatus.CREATED);
  }

  @Test
  void cancel_works_before_fulfilment_and_is_rejected_while_fulfilling() {
    Order paid = orderIn(OrderStatus.PAID);
    paid.cancel();
    assertThat(paid.status()).isEqualTo(OrderStatus.CANCELLED);

    Order fulfilling = orderIn(OrderStatus.FULFILLING);
    assertThatThrownBy(fulfilling::cancel).isInstanceOf(IllegalTransitionException.class);
  }

  @Test
  void total_can_be_updated_before_completion() {
    Order order = new Order("ord-1", new OrderStateMachine());
    order.updateTotal(1999L);
    assertThat(order.totalMinorUnits()).isEqualTo(1999L);
  }

  @Test
  void completed_order_rejects_every_mutator() {
    Order order = orderIn(OrderStatus.COMPLETED);
    assertThatThrownBy(order::cancel).isInstanceOf(OrderCompletedException.class);
    assertThatThrownBy(() -> order.transitionTo(OrderStatus.FAILED))
        .isInstanceOf(OrderCompletedException.class);
    assertThatThrownBy(() -> order.updateTotal(5L)).isInstanceOf(OrderCompletedException.class);
    assertThat(order.status()).isEqualTo(OrderStatus.COMPLETED);
    assertThat(order.totalMinorUnits()).isZero();
  }
}
