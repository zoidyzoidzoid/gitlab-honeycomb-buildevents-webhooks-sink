#!/usr/bin/env bash
set -efuo pipefail
no_proxy="*" curl -v --header "X-Gitlab-Event: Pipeline Hook" http://localhost:8080/api/message -d @pipeline.json --header "Content-Type: application/json"
no_proxy="*" curl -v --header "X-Gitlab-Event: Job Hook" http://localhost:8080/api/message -d @job.json --header "Content-Type: application/json"
