-- Phase 13: per-node engine + extra inbounds for multi-protocol mixed nodes.
ALTER TABLE `nodes`
  ADD COLUMN `engine`   VARCHAR(16) NOT NULL DEFAULT 'xray' AFTER `status`,
  ADD COLUMN `inbounds` JSON        NULL     AFTER `engine`;
