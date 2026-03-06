CREATE TABLE IF NOT EXISTS events (
    id         INT AUTO_INCREMENT PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    location   VARCHAR(255) NOT NULL,
    date       DATETIME     NOT NULL,
    capacity     INT            NOT NULL,
    ticket_price DECIMAL(10,2)  NOT NULL,
    sold_count   INT            NOT NULL DEFAULT 0,
    created_at DATETIME     NOT NULL,
    updated_at DATETIME     NOT NULL
);

CREATE TABLE IF NOT EXISTS purchases (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    buyer_email VARCHAR(255)   NOT NULL,
    event_id    INT            NOT NULL,
    quantity    INT            NOT NULL,
    total_price DECIMAL(10, 2) NOT NULL,
    created_at  DATETIME       NOT NULL,
    FOREIGN KEY (event_id) REFERENCES events(id)
) AUTO_INCREMENT = 1000;

CREATE TABLE IF NOT EXISTS tickets (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    code        VARCHAR(36)  NOT NULL UNIQUE,
    event_id    INT          NOT NULL,
    purchase_id INT          NOT NULL,
    status      VARCHAR(20)  NOT NULL DEFAULT 'emitted',
    used_at     DATETIME     NULL,
    created_at  DATETIME     NOT NULL,
    updated_at  DATETIME     NOT NULL,
    FOREIGN KEY (event_id) REFERENCES events(id),
    FOREIGN KEY (purchase_id) REFERENCES purchases(id),
    INDEX idx_tickets_code (code),
    INDEX idx_tickets_event_id (event_id)
) AUTO_INCREMENT = 1000;
