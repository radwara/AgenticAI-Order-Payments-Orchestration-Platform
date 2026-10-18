package com.agenticpay.order.domain;

/** Lifecycle states of an order. COMPLETED, CANCELLED and FAILED are terminal. */
public enum OrderStatus {
  CREATED,
  PRICED,
  RESERVED,
  PAID,
  FULFILLING,
  COMPLETED,
  CANCELLED,
  FAILED
}
