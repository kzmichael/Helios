-- Notifications are requested at-least-once, so the alert engine must be able to
-- insert the same delivery twice without creating a second row. The key is
-- "<alert id>:<channel id>", set by the alert engine.
CREATE UNIQUE INDEX nd_idempotency_uniq ON notification_delivery (idempotency_key);
