# GitLab Honeycomb Buildevents Webhooks Sink

Server to create honeycomb.io buildevents from GitLab CI pipeline and job webhooks.

If you set this up, and add the public URL with the path of `/api/message`, to your GitLab project, and enable sending Pipeline and Job webhooks to it, then you will soon be able to see the events for them in your Honeycomb dataset.

We support the same environment variables as [buildevents](https://github.com/honeycombio/buildevents), most importantly `BUILDEVENT_APIKEY`. They are all documented [here](https://github.com/honeycombio/buildevents#environment-variables).

Check out the buildevents project that hugely influenced this project at https://github.com/honeycombio/buildevents . Another way to solve this would be doing something like the Circle CI's API usage with the [`buildevents watch`](https://github.com/honeycombio/buildevents#watch) command.

## Overview

### Basic usage

### Advanced usage

We use the same logic as [buildevents](https://github.com/honeycombio/buildevents) to generate trace IDs, so if you use the buildevents CLI to instrument steps and commands in your CI pipelines, those will show up in your pipeline traces in Honeycomb too!

#### Example

Work in Progress example of using `buildevents cmd` here: https://gitlab.com/zoidyzoidzoid/sample-gitlab-project/-/blob/master/.gitlab-ci.yml

## Details

```
GET /healthz: healthcheck

POST /api/message: receive webhooks
```

[GitLab Pipeline Webhooks Documentation](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#pipeline-events)
[GitLab Job Webhooks Documentation](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#job-events)

https://github.com/honeycombio/buildevents/blob/06856ef24981b796af33bcf03e004b9cba4cb687/common.go#L68-L77
