ALTER TABLE `orders`
  DROP KEY `idx_orders_manual_review`,
  DROP COLUMN `risk_score`,
  DROP COLUMN `requires_manual_review`;

ALTER TABLE `users`
  DROP COLUMN `last_login_country`,
  DROP COLUMN `subscribe_token_rotated_at`;

DROP TABLE `login_devices`;
