# GitLab Honeycomb Buildevents Webhooks Sink

GET /healthz: healthcheck

POST /api/message: receive webhooks

Webhook endpoint to create honeycomb.io buildevents from GitLab CI pipeline and job webhooks

https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#pipeline-events
https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#job-events
https://github.com/honeycombio/buildevents/blob/06856ef24981b796af33bcf03e004b9cba4cb687/common.go#L68-L77
