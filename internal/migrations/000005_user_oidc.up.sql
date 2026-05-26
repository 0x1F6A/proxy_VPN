-- Phase 14-C: OIDC SSO subject linkage on users.
ALTER TABLE `users`
  ADD COLUMN `oidc_subject` VARCHAR(255) NULL AFTER `email`,
  ADD UNIQUE KEY `uk_users_oidc_subject` (`oidc_subject`);
