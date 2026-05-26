-- proxy_VPN 第一版数据库 schema (MySQL 8.0+)
--
-- 约定:
--   * 字符集 utf8mb4 / 排序 utf8mb4_0900_ai_ci
--   * 引擎 InnoDB
--   * 主键统一 BIGINT UNSIGNED AUTO_INCREMENT，对外暴露用 uuid/no
--   * 所有时间列使用 DATETIME (UTC) + 默认 CURRENT_TIMESTAMP
--   * 金额一律 DECIMAL(12,2) CNY；USDT 用 DECIMAL(20,6)
--   * 字节数用 BIGINT UNSIGNED
--   * 软删除统一 deleted_at DATETIME NULL，配合 GORM
--   * 命名: 表名 snake_case 复数；外键列 <entity>_id (不建实际 FK，靠应用层保证)

-- --------------------------------------------------------------------
-- 1. 用户体系
-- --------------------------------------------------------------------
CREATE TABLE `users` (
  `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `email`              VARCHAR(128)    NOT NULL,
  `password_hash`      VARCHAR(255)    NOT NULL COMMENT 'argon2id',
  `uuid`               CHAR(36)        NOT NULL COMMENT '代理鉴权 UUID',
  `role`               VARCHAR(16)     NOT NULL DEFAULT 'user' COMMENT 'user|admin|ops|finance',
  `status`             TINYINT         NOT NULL DEFAULT 1 COMMENT '0=禁用 1=正常 2=待激活',
  `balance_cny`        DECIMAL(12,2)   NOT NULL DEFAULT 0.00,
  `plan_id`            BIGINT UNSIGNED NULL,
  `plan_expire_at`     DATETIME        NULL,
  `traffic_total`      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前周期配额 bytes',
  `traffic_used`       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前周期已用 bytes',
  `traffic_reset_at`   DATETIME        NULL COMMENT '下次流量重置时间',
  `device_limit`       INT UNSIGNED    NOT NULL DEFAULT 3,
  `subscription_token` CHAR(40)        NOT NULL COMMENT '订阅链接 token',
  `totp_secret`        VARCHAR(64)     NULL,
  `totp_enabled`       TINYINT(1)      NOT NULL DEFAULT 0,
  `invited_by`         BIGINT UNSIGNED NULL,
  `invite_code`        CHAR(8)         NOT NULL,
  `last_login_at`      DATETIME        NULL,
  `last_login_ip`      VARCHAR(45)     NULL,
  `created_at`         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at`         DATETIME        NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_users_email`  (`email`),
  UNIQUE KEY `uk_users_uuid`   (`uuid`),
  UNIQUE KEY `uk_users_sub`    (`subscription_token`),
  UNIQUE KEY `uk_users_invite` (`invite_code`),
  KEY `idx_users_plan`         (`plan_id`),
  KEY `idx_users_invited_by`   (`invited_by`),
  KEY `idx_users_deleted_at`   (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `refresh_tokens` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `token_hash` CHAR(64)        NOT NULL COMMENT 'sha256(refresh_token)',
  `user_agent` VARCHAR(255)    NULL,
  `ip`         VARCHAR(45)     NULL,
  `expires_at` DATETIME        NOT NULL,
  `revoked_at` DATETIME        NULL,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_rtoken_hash` (`token_hash`),
  KEY `idx_rtoken_user` (`user_id`),
  KEY `idx_rtoken_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `email_codes` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `email`      VARCHAR(128)    NOT NULL,
  `scene`      VARCHAR(32)     NOT NULL COMMENT 'register|reset_password|change_email',
  `code_hash`  CHAR(64)        NOT NULL,
  `attempts`   TINYINT UNSIGNED NOT NULL DEFAULT 0,
  `expires_at` DATETIME        NOT NULL,
  `used_at`    DATETIME        NULL,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_email_codes_email_scene` (`email`, `scene`),
  KEY `idx_email_codes_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `login_logs` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NULL,
  `email`      VARCHAR(128)    NOT NULL,
  `success`    TINYINT(1)      NOT NULL,
  `ip`         VARCHAR(45)     NOT NULL,
  `user_agent` VARCHAR(255)    NULL,
  `reason`     VARCHAR(64)     NULL,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_login_user_time` (`user_id`, `created_at`),
  KEY `idx_login_email_time` (`email`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `sessions` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`        BIGINT UNSIGNED NOT NULL,
  `node_id`        BIGINT UNSIGNED NOT NULL,
  `device_fp`      VARCHAR(64)     NOT NULL COMMENT '设备指纹',
  `ip`             VARCHAR(45)     NOT NULL,
  `last_active_at` DATETIME        NOT NULL,
  `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_user_device` (`user_id`, `device_fp`),
  KEY `idx_session_user_active` (`user_id`, `last_active_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------------------
-- 2. 套餐 / 流量包 / 优惠券
-- --------------------------------------------------------------------
CREATE TABLE `node_groups` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`       VARCHAR(64)     NOT NULL,
  `level`      INT             NOT NULL DEFAULT 0 COMMENT '等级，越大权限越高',
  `remark`     VARCHAR(255)    NULL,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_node_groups_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `plans` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`             VARCHAR(64)     NOT NULL,
  `description`      VARCHAR(512)    NULL,
  `price_cny`        DECIMAL(12,2)   NOT NULL,
  `duration_days`    INT UNSIGNED    NOT NULL,
  `traffic_gb`       INT UNSIGNED    NOT NULL COMMENT '总流量 GB',
  `device_limit`     INT UNSIGNED    NOT NULL DEFAULT 3,
  `speed_limit_mbps` INT UNSIGNED    NOT NULL DEFAULT 0 COMMENT '0=不限速',
  `node_group_id`    BIGINT UNSIGNED NOT NULL,
  `tags`             VARCHAR(255)    NULL COMMENT '逗号分隔, 热销/推荐/限定',
  `sort`             INT             NOT NULL DEFAULT 0,
  `status`           TINYINT         NOT NULL DEFAULT 1 COMMENT '0=下架 1=上架',
  `created_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at`       DATETIME        NULL,
  PRIMARY KEY (`id`),
  KEY `idx_plans_status_sort` (`status`, `sort`),
  KEY `idx_plans_group` (`node_group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `data_packs` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`         VARCHAR(64)     NOT NULL,
  `price_cny`    DECIMAL(12,2)   NOT NULL,
  `traffic_gb`   INT UNSIGNED    NOT NULL,
  `valid_days`   INT UNSIGNED    NOT NULL COMMENT '加成到套餐到期日 / 独立有效期',
  `attach_mode`  TINYINT         NOT NULL DEFAULT 1 COMMENT '1=并入当前周期 2=独立有效期',
  `sort`         INT             NOT NULL DEFAULT 0,
  `status`       TINYINT         NOT NULL DEFAULT 1,
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_packs_status_sort` (`status`, `sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `coupons` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `code`           VARCHAR(32)     NOT NULL,
  `discount_type`  TINYINT         NOT NULL COMMENT '1=固定金额 2=百分比',
  `discount_value` DECIMAL(12,2)   NOT NULL,
  `min_amount`     DECIMAL(12,2)   NOT NULL DEFAULT 0.00,
  `applicable`     VARCHAR(32)     NOT NULL DEFAULT 'plan' COMMENT 'plan|pack|all',
  `total_quota`    INT UNSIGNED    NOT NULL DEFAULT 0 COMMENT '0=无限',
  `used_count`     INT UNSIGNED    NOT NULL DEFAULT 0,
  `per_user_limit` INT UNSIGNED    NOT NULL DEFAULT 1,
  `starts_at`      DATETIME        NULL,
  `expires_at`     DATETIME        NULL,
  `status`         TINYINT         NOT NULL DEFAULT 1,
  `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_coupons_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------------------
-- 3. 订单与支付
-- --------------------------------------------------------------------
CREATE TABLE `orders` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_no`         CHAR(24)        NOT NULL COMMENT '业务单号',
  `user_id`          BIGINT UNSIGNED NOT NULL,
  `type`             VARCHAR(16)     NOT NULL COMMENT 'plan|pack|topup',
  `target_id`        BIGINT UNSIGNED NOT NULL COMMENT 'plan_id/pack_id, topup=0',
  `target_snapshot`  JSON            NULL COMMENT '下单时套餐/包快照',
  `amount_cny`       DECIMAL(12,2)   NOT NULL,
  `discount_cny`     DECIMAL(12,2)   NOT NULL DEFAULT 0.00,
  `paid_cny`         DECIMAL(12,2)   NOT NULL DEFAULT 0.00,
  `coupon_code`      VARCHAR(32)     NULL,
  `pay_method`       VARCHAR(16)     NOT NULL COMMENT 'alipay|wechat|usdt_trc20|balance',
  `pay_channel_no`   VARCHAR(64)     NULL COMMENT '通道交易号',
  `pay_extra`        JSON            NULL COMMENT '通道返回的关键字段',
  `status`           VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending|paid|cancelled|expired|refunded|partial_refund',
  `expire_at`        DATETIME        NOT NULL,
  `paid_at`          DATETIME        NULL,
  `refunded_at`      DATETIME        NULL,
  `idempotency_key`  CHAR(36)        NOT NULL,
  `client_ip`        VARCHAR(45)     NULL,
  `remark`           VARCHAR(255)    NULL,
  `created_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_orders_no`  (`order_no`),
  UNIQUE KEY `uk_orders_idem`(`idempotency_key`),
  KEY `idx_orders_user_status` (`user_id`, `status`),
  KEY `idx_orders_status_expire` (`status`, `expire_at`),
  KEY `idx_orders_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `payment_callbacks` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_no`     CHAR(24)        NULL,
  `pay_method`   VARCHAR(16)     NOT NULL,
  `channel_no`   VARCHAR(64)     NULL,
  `verified`     TINYINT(1)      NOT NULL DEFAULT 0,
  `raw_payload`  MEDIUMTEXT      NOT NULL,
  `processed_at` DATETIME        NULL,
  `error`        VARCHAR(512)    NULL,
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_pay_cb_order` (`order_no`),
  KEY `idx_pay_cb_method_channel` (`pay_method`, `channel_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `usdt_addresses` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `chain`      VARCHAR(16)     NOT NULL DEFAULT 'TRON' COMMENT 'TRON|ETH|BSC',
  `address`    VARCHAR(80)     NOT NULL,
  `in_use`     TINYINT(1)      NOT NULL DEFAULT 1,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_usdt_addr_chain` (`chain`, `address`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `usdt_payments` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_no`     CHAR(24)        NULL,
  `chain`        VARCHAR(16)     NOT NULL,
  `address`      VARCHAR(80)     NOT NULL,
  `amount_usdt`  DECIMAL(20,6)   NOT NULL,
  `tx_hash`      VARCHAR(80)     NOT NULL,
  `block_height` BIGINT UNSIGNED NULL,
  `confirmed_at` DATETIME        NULL,
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_usdt_pay_tx` (`tx_hash`),
  KEY `idx_usdt_pay_order` (`order_no`),
  KEY `idx_usdt_pay_addr_amount` (`address`, `amount_usdt`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `refunds` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `refund_no`       CHAR(24)        NOT NULL,
  `order_no`        CHAR(24)        NOT NULL,
  `amount_cny`      DECIMAL(12,2)   NOT NULL,
  `reason`          VARCHAR(255)    NULL,
  `status`          VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending|success|failed',
  `channel_refund_no` VARCHAR(64)   NULL,
  `operator_id`     BIGINT UNSIGNED NULL,
  `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_refund_no` (`refund_no`),
  KEY `idx_refund_order` (`order_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `balance_logs` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`      BIGINT UNSIGNED NOT NULL,
  `amount_cny`   DECIMAL(12,2)   NOT NULL COMMENT '正=入账 负=扣费',
  `balance_after` DECIMAL(12,2)  NOT NULL,
  `biz_type`     VARCHAR(32)     NOT NULL COMMENT 'topup|consume|refund|invite_reward|adjust',
  `biz_ref`      VARCHAR(64)     NULL COMMENT '关联订单号/退款号等',
  `remark`       VARCHAR(255)    NULL,
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_balance_user_time` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------------------
-- 4. 节点
-- --------------------------------------------------------------------
CREATE TABLE `nodes` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`              VARCHAR(64)     NOT NULL,
  `region`            VARCHAR(8)      NOT NULL COMMENT 'US|JP|HK|SG|...',
  `tags`              VARCHAR(255)    NULL,
  `node_group_id`     BIGINT UNSIGNED NOT NULL,
  `protocol`          VARCHAR(32)     NOT NULL COMMENT 'vless-reality|trojan|hysteria2|ss-2022',
  `address`           VARCHAR(255)    NOT NULL,
  `port`              INT UNSIGNED    NOT NULL,
  `tls_config`        JSON            NULL,
  `transport`         VARCHAR(32)     NULL COMMENT 'tcp|ws|grpc|xhttp',
  `transport_config`  JSON            NULL,
  `rate_multiplier`   DECIMAL(4,2)    NOT NULL DEFAULT 1.00 COMMENT '流量倍率',
  `node_token_hash`   CHAR(64)        NOT NULL COMMENT 'sha256(node_token)',
  `online`            TINYINT(1)      NOT NULL DEFAULT 0,
  `last_heartbeat_at` DATETIME        NULL,
  `cpu_percent`       DECIMAL(5,2)    NULL,
  `mem_percent`       DECIMAL(5,2)    NULL,
  `bandwidth_in_bps`  BIGINT UNSIGNED NULL,
  `bandwidth_out_bps` BIGINT UNSIGNED NULL,
  `online_users`      INT UNSIGNED    NOT NULL DEFAULT 0,
  `sort`              INT             NOT NULL DEFAULT 0,
  `status`            TINYINT         NOT NULL DEFAULT 1 COMMENT '0=禁用 1=正常 2=维护中',
  `created_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at`        DATETIME        NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_nodes_token` (`node_token_hash`),
  KEY `idx_nodes_group_status_sort` (`node_group_id`, `status`, `sort`),
  KEY `idx_nodes_region` (`region`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `plan_node_groups` (
  `plan_id`       BIGINT UNSIGNED NOT NULL,
  `node_group_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`plan_id`, `node_group_id`),
  KEY `idx_pn_group` (`node_group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------------------
-- 5. 流量统计 (热数据。冷数据走 ClickHouse)
-- --------------------------------------------------------------------
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

CREATE TABLE `traffic_resets` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`      BIGINT UNSIGNED NOT NULL,
  `reset_at`     DATETIME        NOT NULL,
  `before_used`  BIGINT UNSIGNED NOT NULL,
  `before_total` BIGINT UNSIGNED NOT NULL,
  `reason`       VARCHAR(64)     NOT NULL COMMENT 'monthly|plan_renewed|plan_upgraded|admin',
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_traffic_reset_user_time` (`user_id`, `reset_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------------------
-- 6. 邀请返佣
-- --------------------------------------------------------------------
CREATE TABLE `invite_commissions` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `inviter_id`   BIGINT UNSIGNED NOT NULL,
  `invitee_id`   BIGINT UNSIGNED NOT NULL,
  `order_no`     CHAR(24)        NOT NULL,
  `amount_cny`   DECIMAL(12,2)   NOT NULL,
  `rate`         DECIMAL(5,4)    NOT NULL COMMENT '返佣比例 0~1',
  `status`       VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending|settled|cancelled',
  `settled_at`   DATETIME        NULL,
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_invite_order` (`order_no`),
  KEY `idx_invite_inviter` (`inviter_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `withdrawals` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `withdraw_no`  CHAR(24)        NOT NULL,
  `user_id`      BIGINT UNSIGNED NOT NULL,
  `amount_cny`   DECIMAL(12,2)   NOT NULL,
  `method`       VARCHAR(16)     NOT NULL COMMENT 'balance|usdt|alipay',
  `target_addr`  VARCHAR(255)    NULL,
  `status`       VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending|approved|rejected|paid',
  `operator_id`  BIGINT UNSIGNED NULL,
  `remark`       VARCHAR(255)    NULL,
  `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_withdraw_no` (`withdraw_no`),
  KEY `idx_withdraw_user_status` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------------------
-- 7. 工单
-- --------------------------------------------------------------------
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

-- --------------------------------------------------------------------
-- 8. 系统 / 公告 / 审计
-- --------------------------------------------------------------------
CREATE TABLE `notices` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `title`      VARCHAR(128)    NOT NULL,
  `content`    MEDIUMTEXT      NOT NULL,
  `pinned`     TINYINT(1)      NOT NULL DEFAULT 0,
  `published_at` DATETIME      NULL,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_notices_published` (`published_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `settings` (
  `key`        VARCHAR(64)  NOT NULL,
  `value`      MEDIUMTEXT   NOT NULL,
  `is_secret`  TINYINT(1)   NOT NULL DEFAULT 0,
  `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `admin_audit_logs` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `admin_id`   BIGINT UNSIGNED NOT NULL,
  `action`     VARCHAR(64)     NOT NULL,
  `target`     VARCHAR(128)    NULL,
  `before`     JSON            NULL,
  `after`      JSON            NULL,
  `ip`         VARCHAR(45)     NULL,
  `user_agent` VARCHAR(255)    NULL,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_audit_admin_time` (`admin_id`, `created_at`),
  KEY `idx_audit_action_time` (`action`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `webhook_endpoints` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `url`        VARCHAR(512)    NOT NULL,
  `secret`     VARCHAR(64)     NOT NULL,
  `events`     VARCHAR(512)    NOT NULL COMMENT '逗号分隔事件名',
  `enabled`    TINYINT(1)      NOT NULL DEFAULT 1,
  `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_webhook_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
