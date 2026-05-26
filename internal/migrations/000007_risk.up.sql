-- Phase 15-A: 风控反滥用 — 设备追踪 + 订阅 token 治理 + 订单风控字段。

-- login_devices：每次登录成功登记一台设备指纹，admin 可强制下线。
CREATE TABLE `login_devices` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`       BIGINT UNSIGNED NOT NULL,
  `fp_hash`       CHAR(64)        NOT NULL COMMENT 'sha256(ua + accept-language + ip/24)',
  `ip`            VARCHAR(64)     NOT NULL DEFAULT '',
  `user_agent`    VARCHAR(255)    NOT NULL DEFAULT '',
  `country`       VARCHAR(8)      NOT NULL DEFAULT '',
  `first_seen_at` DATETIME        NOT NULL,
  `last_seen_at`  DATETIME        NOT NULL,
  `revoked_at`    DATETIME        NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_login_devices_user_fp` (`user_id`, `fp_hash`),
  KEY `idx_login_devices_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- users 表新增订阅 token 轮换时间 / 上次登录国家。
ALTER TABLE `users`
  ADD COLUMN `subscribe_token_rotated_at` DATETIME NULL AFTER `subscription_token`,
  ADD COLUMN `last_login_country` VARCHAR(8) NOT NULL DEFAULT '' AFTER `last_login_ip`;

-- orders 表新增风控字段：需人工审核 / 风险分。
ALTER TABLE `orders`
  ADD COLUMN `requires_manual_review` TINYINT(1) NOT NULL DEFAULT 0 AFTER `status`,
  ADD COLUMN `risk_score` INT NOT NULL DEFAULT 0 AFTER `requires_manual_review`,
  ADD KEY `idx_orders_manual_review` (`requires_manual_review`);
