-- Phase 14-C: SLA self-probes + daily rollups.
CREATE TABLE `sla_probes` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `ts`         DATETIME(3)     NOT NULL,
  `region`     VARCHAR(32)     NOT NULL DEFAULT '',
  `target`     VARCHAR(64)     NOT NULL COMMENT 'api|admin|user-web|readyz target',
  `success`    TINYINT(1)      NOT NULL,
  `latency_ms` INT UNSIGNED    NOT NULL DEFAULT 0,
  `err`        VARCHAR(255)    NULL,
  PRIMARY KEY (`id`),
  KEY `idx_sla_probes_ts`           (`ts`),
  KEY `idx_sla_probes_target_ts`    (`target`, `ts`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `sla_daily` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `day`         DATE            NOT NULL,
  `region`      VARCHAR(32)     NOT NULL DEFAULT '',
  `target`      VARCHAR(64)     NOT NULL,
  `success_cnt` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `fail_cnt`    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `p50_ms`      INT UNSIGNED    NOT NULL DEFAULT 0,
  `p95_ms`      INT UNSIGNED    NOT NULL DEFAULT 0,
  `p99_ms`      INT UNSIGNED    NOT NULL DEFAULT 0,
  `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sla_daily_day_region_target` (`day`,`region`,`target`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
