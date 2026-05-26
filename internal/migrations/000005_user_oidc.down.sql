ALTER TABLE `users`
  DROP INDEX `uk_users_oidc_subject`,
  DROP COLUMN `oidc_subject`;
