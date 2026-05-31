ALTER TABLE `users` DROP COLUMN `locale`;
DROP TABLE IF EXISTS `ticket_messages`;
DROP TABLE IF EXISTS `tickets`;

-- Recreate the legacy tickets / ticket_replies schema from 000001 so a clean
-- rollback leaves the database in the state expected by earlier migrations
-- (mirrors the 000003 traffic_daily rollback convention).
CREATE TABLE `tickets` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `subject`    VARCHAR(128)    NOT NULL,
  `category`   VARCHAR(32)     NOT NULL DEFAULT 'general' COMMENT 'general|billing|tech|abuse',
  `status`     VARCHAR(16)     NOT NULL DEFAULT 'open' COMMENT 'open|pending|closed',
  `priority`   TINYINT         NOT NULL DEFAULT 1 COMMENT '0=low 1=normal 2=high',
  `last_reply_at` DATETIME     NULL,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_tickets_user_status` (`user_id`, `status`),
  KEY `idx_tickets_status_time` (`status`, `last_reply_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `ticket_replies` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `ticket_id`   BIGINT UNSIGNED NOT NULL,
  `sender_type` TINYINT         NOT NULL COMMENT '1=user 2=staff',
  `sender_id`   BIGINT UNSIGNED NOT NULL,
  `content`     MEDIUMTEXT      NOT NULL,
  `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ticket_replies_ticket_time` (`ticket_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
