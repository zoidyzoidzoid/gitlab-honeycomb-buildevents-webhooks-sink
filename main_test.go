package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/honeycombio/libhoney-go"
	"github.com/honeycombio/libhoney-go/transmission"
)

func Test_createEvent(t *testing.T) {
	defer libhoney.Close()
	var config libhoney.Config
	wantedFields := map[string]string{
		"ci_provider":  "GitLab-CI",
		"meta.version": "dev",
	}
	wantedUserAgentAddition := "buildevents/dev (GitLab-CI)"
	t.Run("valid event", func(t *testing.T) {
		got, err := createEvent(&config)
		if err != nil {
			t.Errorf("failed to create event: %s", err)
		}

		for k, v := range got.Fields() {
			if v != wantedFields[k] {
				t.Errorf("event fields key '%s' = %v, want %v", k, v, wantedFields[k])
			}
		}
		if libhoney.UserAgentAddition != wantedUserAgentAddition {
			t.Errorf("user agent addition = %v, want %v", libhoney.UserAgentAddition, wantedUserAgentAddition)
		}
	})
}

func Test_createTraceFromPipeline(t *testing.T) {
	defer libhoney.Close()
	var config libhoney.Config
	tests := []struct {
		name     string
		pipeline Pipeline
		want     *libhoney.Event
		wantErr  bool
	}{
		// TODO: Add test cases.
		{
			name: "created pipeline doesn't create an event",
			pipeline: Pipeline{
				ObjectAttributes: PipelineObjectAttributes{
					Status: "created",
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "running pipeline doesn't create an event",
			pipeline: Pipeline{
				ObjectAttributes: PipelineObjectAttributes{
					Status: "running",
				},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createTraceFromPipeline(&config, tt.pipeline)
			if (err != nil) != tt.wantErr {
				t.Errorf("createTraceFromPipeline() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createTraceFromPipeline() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestHandler(t *testing.T) {
	defer libhoney.Close()
	var config libhoney.Config
	mockSender := &transmission.MockSender{}
	config.Transmission = mockSender
	tests := []struct {
		name      string
		eventType string
		hook      interface{}
	}{
		{
			name:      "finished pipeline creates an event",
			eventType: "Pipeline Hook",
			hook: Pipeline{
				MergeRequest: MergeRequest{
					Iid:             1,
					SourceBranch:    "feature-branch-1",
					SourceProjectID: 1,
				},
				ObjectAttributes: PipelineObjectAttributes{
					ID:             1,
					Status:         "success",
					Ref:            "main",
					Duration:       5,
					QueuedDuration: 2,
					CreatedAt:      time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
				},
				Project: Project{
					WebURL: "https://gitlab.com/zoidyzoidzoid/sample-gitlab-project",
				},
			},
		},
		{
			name:      "finished job creates an event",
			eventType: "Job Hook",
			hook: Job{
				PipelineID:          1,
				BuildID:             1,
				BuildStatus:         "success",
				BuildName:           "build",
				Ref:                 "main",
				BuildDuration:       5,
				BuildQueuedDuration: 2,
				BuildStartedAt:      time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
				Repository: Repository{
					Homepage: "https://gitlab.com/zoidyzoidzoid/sample-gitlab-project",
				},
				Runner: Runner{
					Description: "shared-runners-manager-5.gitlab.com",
					ID:          380986,
					Tags: []string{
						"docker",
						"gce",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := json.Marshal(tt.hook)
			if err != nil {
				t.Errorf("unexpected error marshalling request body: %v", err)
			}
			bodyReader := strings.NewReader(string(marshalled))
			req := httptest.NewRequest(http.MethodPost, "/api/message", bodyReader)
			req.Header.Set("X-Gitlab-Event", tt.eventType)
			w := httptest.NewRecorder()
			handleRequest(&config, w, req)
			res := w.Result()
			defer res.Body.Close()
			data, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Errorf("expected error to be nil got %v", err)
			}
			wanted := "Thanks!\n"
			if string(data) != wanted {
				t.Errorf("expected %+v got %+v", wanted, string(data))
			}
			// if (err != nil) != tt.wantErr {
			// 	t.Errorf("createTraceFromPipeline() error = %v, wantErr %v", err, tt.wantErr)
			// 	return
			// }
			// if !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("createTraceFromPipeline() = %v, want %v", got, tt.want)
			// }
		})
	}
}
