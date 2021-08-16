package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/honeycombio/libhoney-go"
)

func Test_parseTime(t *testing.T) {
	type args struct {
		dt string
	}
	dt1, err := time.Parse(time.RFC3339, "2021-08-13T11:06:11Z")
	if err != nil {
		t.Fatalf("parsing wanted datetimes failed: %s", err)
	}
	dt2, err := time.Parse(time.RFC3339, "2021-08-16T15:31:09+02:00")
	if err != nil {
		t.Fatalf("parsing wanted datetimes failed: %s", err)
	}
	tests := []struct {
		name    string
		args    args
		want    *time.Time
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "valid external time",
			args: args{
				dt: "2021-08-13 11:06:11 UTC",
			},
			want:    &dt1,
			wantErr: false,
		},
		{
			name: "valid internal time",
			args: args{
				dt: "2021-08-16 15:31:09 +0200",
			},
			want:    &dt2,
			wantErr: false,
		},
		{
			name: "invalid external time",
			args: args{
				dt: "2021-08-13 11:06:11 UTCC",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTime(tt.args.dt)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_createEvent(t *testing.T) {
	defer libhoney.Close()
	var config libhoney.Config
	wantedFields := make(map[string]string)
	wantedFields["ci_provider"] = "GitLab-CI"
	wantedFields["meta.version"] = "dev"
	wantedUserAgentAddition := "buildevents/dev (GitLab-CI)"
	t.Run("valid event", func(t *testing.T) {
		got := createEvent(&config)
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
