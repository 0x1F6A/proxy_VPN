ALTER TABLE `users`
  DROP COLUMN `rate_bps_up`,
  DROP COLUMN `rate_bps_down`,
  DROP COLUMN `banned`;

DROP TABLE IF EXISTS `usage_event_fallback`;
DROP TABLE IF EXISTS `traffic_daily`;

-- Recreate the legacy traffic_daily schema from 000001 so a clean rollback
-- leaves the database in the state expected by earlier migrations.
CREATE TABLE `traffic_daily` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`        BIGINT UNSIGNED NOT NULL,
  `node_id`        BIGINT UNSIGNED NOT NULL,
  `stat_date`      DATE            NOT NULL,
  `upload_bytes`   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `download_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_traffic_daily` (`user_id`, `node_id`, `stat_date`),
  KEY `idx_traffic_date` (`stat_date`),
  KEY `idx_traffic_user_date` (`user_id`, `stat_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
