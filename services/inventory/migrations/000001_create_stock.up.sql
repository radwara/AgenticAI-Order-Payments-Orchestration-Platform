CREATE TABLE stock (
    sku        text        NOT NULL,
    location   text        NOT NULL,
    on_hand    bigint      NOT NULL DEFAULT 0,
    reserved   bigint      NOT NULL DEFAULT 0,
    version    bigint      NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (sku, location),
    CONSTRAINT stock_reserved_within_on_hand CHECK (reserved >= 0 AND reserved <= on_hand)
);
