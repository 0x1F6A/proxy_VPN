-- Phase 6 schema additions: per-user traffic caps + ban flag, daily rollup,
-- and an event fallback table used only when ClickHouse is unavailable.

ALTER TABLE `users`
  ADD COLUMN `rate_bps_up`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=unlimited',
  ADD COLUMN `rate_bps_down` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=unlimited',
  ADD COLUMN `banned`        TINYINT(1)      NOT NULL DEFAULT 0 COMMENT 'over-quota or admin-set';

-- The legacy `traffic_daily` table shipped in 000001 used a different schema
-- (id PK, stat_date, upload_bytes, download_bytes) intended for an earlier
-- design that bundled node_id into the key. Phase 6 collapses to a much
-- smaller per-user/per-day rollup, so we drop and recreate to match the
-- shape the gormrepo expects.
DROP TABLE IF EXISTS `traffic_daily`;
CREATE TABLE `traffic_daily` (
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `day`        DATE            NOT NULL,
  `up_bytes`   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `down_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`, `day`),
  KEY `idx_traffic_daily_day` (`day`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='per-user per-day traffic rollup';

CREATE TABLE IF NOT EXISTS `usage_event_fallback` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `ts`         DATETIME        NOT NULL,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `node_id`    BIGINT UNSIGNED NOT NULL,
  `protocol`   VARCHAR(32)     NOT NULL DEFAULT '',
  `up_bytes`   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `down_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `flushed`    TINYINT(1)      NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_uef_user_ts` (`user_id`, `ts`),
  KEY `idx_uef_flushed` (`flushed`, `ts`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT='write-when-CH-down; replayed by traffic:flush_ch_buffer task';
