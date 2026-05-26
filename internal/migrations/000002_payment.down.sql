-- Rollback Phase 5
ALTER TABLE `orders` DROP INDEX `uk_orders_user_idem`;
ALTER TABLE `orders` ADD UNIQUE KEY `uk_orders_idem` (`idempotency_key`);

DROP TABLE IF EXISTS `chain_scan_cursor`;
DROP TABLE IF EXISTS `payment_addresses`;
DROP TABLE IF EXISTS `payments`;
