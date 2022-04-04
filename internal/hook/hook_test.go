package hook

import (
	"github.com/honeycombio/libhoney-go"
	"testing"
)

func Test_createEvent(t *testing.T) {
	defer libhoney.Close()
	var config libhoney.Config
	wantedFields := make(map[string]string)
	wantedFields["ci_provider"] = "GitLab-CI"
	wantedFields["meta.version"] = "dev"
	wantedUserAgentAddition := "buildevents/dev (GitLab-CI)"
	t.Run("valid event", func(t *testing.T) {
		l, err := New(Config{
			Version:         "dev",
			ListenAddr:      ":8080",
			HookSecret:      "",
			HoneycombConfig: &config,
		})
		got, err := l.createEvent()
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

//func Test_HandlePipeline(t *testing.T) {
//	defer libhoney.Close()
//	var config libhoney.Config
//	tests := []struct {
//		name     string
//		pipeline gitlab.PipelineEventPayload
//		want     *libhoney.Event
//		wantErr  bool
//	}{
//		// TODO: Add test cases.
//		{
//			name: "created pipeline doesn't create an event",
//			pipeline: Pipeline{
//				ObjectAttributes: PipelineObjectAttributes{
//					Status: "created",
//				},
//			},
//			want:    nil,
//			wantErr: false,
//		},
//		{
//			name: "running pipeline doesn't create an event",
//			pipeline: Pipeline{
//				ObjectAttributes: PipelineObjectAttributes{
//					Status: "running",
//				},
//			},
//			want:    nil,
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			got, err := createTraceFromPipeline(&config, tt.pipeline)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("createTraceFromPipeline() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("createTraceFromPipeline() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
