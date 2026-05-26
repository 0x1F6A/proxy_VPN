-- Phase 5: 真实支付 + USDT 地址池 + 修复 orders 幂等键

-- 1. payments：通道支付凭据 / 状态 / 回调原文
CREATE TABLE `payments` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_no`        CHAR(24)        NOT NULL,
  `user_id`         BIGINT UNSIGNED NOT NULL,
  `channel`         VARCHAR(16)     NOT NULL COMMENT 'alipay|wechat|usdt_trc20',
  `channel_trade_no` VARCHAR(64)    NULL COMMENT '通道交易号，回调后回填',
  `amount_cny`      DECIMAL(12,2)   NOT NULL,
  `amount_token`    DECIMAL(20,6)   NULL COMMENT 'USDT 折算金额',
  `status`          VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending|paid|expired|failed|refunded',
  `qr_or_url`       VARCHAR(1024)   NULL COMMENT '支付二维码内容或跳转链接',
  `address`         VARCHAR(64)     NULL COMMENT 'USDT 收款地址',
  `raw_notify`      MEDIUMTEXT      NULL COMMENT '回调原始负载',
  `paid_at`         DATETIME        NULL,
  `expired_at`      DATETIME        NOT NULL,
  `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_payments_channel_tradeno` (`channel`, `channel_trade_no`),
  KEY `idx_payments_order` (`order_no`),
  KEY `idx_payments_status_expire` (`status`, `expired_at`),
  KEY `idx_payments_address` (`address`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 2. payment_addresses：USDT 一次性收款地址池
CREATE TABLE `payment_addresses` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `channel`      VARCHAR(16)     NOT NULL COMMENT 'usdt_trc20',
  `address`      VARCHAR(64)     NOT NULL,
  `status`       VARCHAR(16)     NOT NULL DEFAULT 'free' COMMENT 'free|allocated|used',
  `order_no`     CHAR(24)        NULL,
  `allocated_at` DATETIME        NULL,
  `released_at`  DATETIME        NULL,
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pa_channel_addr` (`channel`, `address`),
  KEY `idx_pa_status` (`channel`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 3. usdt_scan_cursor：链扫游标
CREATE TABLE `chain_scan_cursor` (
  `chain`       VARCHAR(16)     NOT NULL,
  `last_block`  BIGINT          NOT NULL DEFAULT 0,
  `updated_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`chain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 4. 修复 orders 幂等键：单列全局 UNIQUE → (user_id, idempotency_key) 复合 UNIQUE
ALTER TABLE `orders` DROP INDEX `uk_orders_idem`;
ALTER TABLE `orders` ADD UNIQUE KEY `uk_orders_user_idem` (`user_id`, `idempotency_key`);
