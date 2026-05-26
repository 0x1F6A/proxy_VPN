ALTER TABLE `users`
  DROP COLUMN `rate_bps_up`,
  DROP COLUMN `rate_bps_down`,
  DROP COLUMN `banned`;

DROP TABLE IF EXISTS `usage_event_fallback`;
DROP TABLE IF EXISTS `traffic_daily`;
