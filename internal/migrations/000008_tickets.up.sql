-- Phase 15-C: 工单系统 + 用户 locale。

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
