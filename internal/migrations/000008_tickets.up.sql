-- Phase 15-C: 工单系统 + 用户 locale。
--
-- 000001 曾以旧设计创建 `tickets` / `ticket_replies`（未被任何代码引用）。
-- 本阶段重新设计为 `tickets`（含 assignee_id） + `ticket_messages`，
-- 因此先丢弃旧表再重建，保证全新库与已部署旧库都能一致迁移。
DROP TABLE IF EXISTS `ticket_replies`;
DROP TABLE IF EXISTS `tickets`;

CREATE TABLE `tickets` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`      BIGINT UNSIGNED NOT NULL,
  `subject`      VARCHAR(200)    NOT NULL,
  `category`     VARCHAR(32)     NOT NULL DEFAULT 'general',
  `priority`     VARCHAR(16)     NOT NULL DEFAULT 'normal' COMMENT 'low|normal|high|urgent',
  `status`       VARCHAR(16)     NOT NULL DEFAULT 'open'   COMMENT 'open|pending|resolved|closed',
  `assignee_id`  BIGINT UNSIGNED NULL,
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_tickets_user`     (`user_id`),
  KEY `idx_tickets_status`   (`status`),
  KEY `idx_tickets_assignee` (`assignee_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `ticket_messages` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `ticket_id`   BIGINT UNSIGNED NOT NULL,
  `sender_id`   BIGINT UNSIGNED NOT NULL,
  `sender_type` VARCHAR(8)      NOT NULL COMMENT 'user|admin',
  `body`        TEXT            NOT NULL,
  `attachments` JSON            NULL,
  `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ticket_msgs_ticket` (`ticket_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `users`
  ADD COLUMN `locale` VARCHAR(16) NOT NULL DEFAULT '' AFTER `invite_code`;
