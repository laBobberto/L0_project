CREATE TABLE IF NOT EXISTS deliveries (
                                          id SERIAL PRIMARY KEY,
                                          name VARCHAR(255) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    zip VARCHAR(10) NOT NULL,
    city VARCHAR(100) NOT NULL,
    address VARCHAR(255) NOT NULL,
    region VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL
    );

CREATE TABLE IF NOT EXISTS payments (
                                        id SERIAL PRIMARY KEY,
                                        transaction VARCHAR(255) UNIQUE NOT NULL,
    request_id VARCHAR(255),
    currency VARCHAR(10) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    amount INT NOT NULL,
    payment_dt BIGINT NOT NULL,
    bank VARCHAR(100) NOT NULL,
    delivery_cost INT NOT NULL,
    goods_total INT NOT NULL,
    custom_fee INT NOT NULL
    );

CREATE TABLE IF NOT EXISTS orders (
                                      order_uid VARCHAR(255) PRIMARY KEY,
    track_number VARCHAR(255) UNIQUE NOT NULL,
    entry VARCHAR(50) NOT NULL,
    delivery_id INT REFERENCES deliveries(id) ON DELETE CASCADE,
    payment_id INT REFERENCES payments(id) ON DELETE CASCADE,
    locale VARCHAR(10) NOT NULL,
    internal_signature VARCHAR(255),
    customer_id VARCHAR(255) NOT NULL,
    delivery_service VARCHAR(100) NOT NULL,
    shardkey VARCHAR(10) NOT NULL,
    sm_id INT NOT NULL,
    date_created TIMESTAMPTZ NOT NULL,
    oof_shard VARCHAR(10) NOT NULL
    );

CREATE TABLE IF NOT EXISTS items (
                                     id SERIAL PRIMARY KEY,
                                     order_uid VARCHAR(255) REFERENCES orders(order_uid) ON DELETE CASCADE,
    chrt_id INT NOT NULL,
    track_number VARCHAR(255) NOT NULL,
    price INT NOT NULL,
    rid VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    sale INT NOT NULL,
    size VARCHAR(10) NOT NULL,
    total_price INT NOT NULL,
    nm_id INT NOT NULL,
    brand VARCHAR(100) NOT NULL,
    status INT NOT NULL
    );