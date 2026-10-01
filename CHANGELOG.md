# Changelog

## 0.2.0
- `Client.API`: `WebhooksUpdate` (pause, resume, edit), the delivery log `WebhookDeliveriesList` / `WebhookDeliveriesGet` / `WebhookDeliveriesRetry`, and `WebhooksEventTypes`. Outbound webhooks are retried (1 min, 5 min, 25 min, 2 h, 12 h; 6 attempts), every attempt is recorded, and an endpoint that keeps failing for a day is switched off (`disabledAt`, `disabledReason`; `secronna.webhook_endpoint.disabled` goes to your other endpoints).
