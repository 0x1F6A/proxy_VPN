-- Rollback for 000001_init_schema. Drop in reverse dependency order.

DROP TABLE IF EXISTS `webhook_endpoints`;
DROP TABLE IF EXISTS `admin_audit_logs`;
DROP TABLE IF EXISTS `settings`;
DROP TABLE IF EXISTS `notices`;

DROP TABLE IF EXISTS `ticket_replies`;
DROP TABLE IF EXISTS `tickets`;

DROP TABLE IF EXISTS `withdrawals`;
DROP TABLE IF EXISTS `invite_commissions`;

DROP TABLE IF EXISTS `traffic_resets`;
DROP TABLE IF EXISTS `traffic_daily`;

DROP TABLE IF EXISTS `plan_node_groups`;
DROP TABLE IF EXISTS `nodes`;

DROP TABLE IF EXISTS `balance_logs`;
DROP TABLE IF EXISTS `refunds`;
DROP TABLE IF EXISTS `usdt_payments`;
DROP TABLE IF EXISTS `usdt_addresses`;
DROP TABLE IF EXISTS `payment_callbacks`;
DROP TABLE IF EXISTS `orders`;

DROP TABLE IF EXISTS `coupons`;
DROP TABLE IF EXISTS `data_packs`;
DROP TABLE IF EXISTS `plans`;
DROP TABLE IF EXISTS `node_groups`;

DROP TABLE IF EXISTS `sessions`;
DROP TABLE IF EXISTS `login_logs`;
DROP TABLE IF EXISTS `email_codes`;
DROP TABLE IF EXISTS `refresh_tokens`;
DROP TABLE IF EXISTS `users`;
