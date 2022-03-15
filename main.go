package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/honeycombio/libhoney-go"
	"github.com/honeycombio/libhoney-go/transmission"
	"github.com/spf13/cobra"
)

// Version is the default value that should be overridden in the
// build/release process.
// TODO: Actually set this
var Version = "dev"

func home(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprintf(w, `# GitLab Honeycomb Buildevents Webhooks Sink

GET /healthz: healthcheck

POST /api/message: receive array of notifications
`)
	if err != nil {
		log.Printf("home: failed to write to http response writer: %s", err)
	}
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func createEvent(cfg *libhoney.Config) (*libhoney.Event, error) {
	libhoney.UserAgentAddition = fmt.Sprintf("buildevents/%s (GitLab-CI)", Version)

	if cfg.APIKey == "" {
		cfg.Transmission = &transmission.WriterSender{}
	}

	// TODO: I thinks this should maybe be not using the default package-level client
	ev := libhoney.NewEvent()
	ev.Add(
		map[string]interface{}{
			"ci_provider":  "GitLab-CI",
			"meta.version": Version,
		},
	)
	return ev, nil
}

func createTraceFromPipeline(cfg *libhoney.Config, p Pipeline) (*libhoney.Event, error) {
	// if p.ObjectAttributes.Status == "created" || p.ObjectAttributes.Status == "running" || p.ObjectAttributes.Status == "pending" {
	// 	return nil, nil
	// }
	if p.ObjectAttributes.Duration == 0 || p.ObjectAttributes.Status == "running" {
		return nil, nil
	}
	traceID := fmt.Sprint(p.ObjectAttributes.ID)
	ev, err := createEvent(cfg)
	if err != nil {
		return nil, err
	}

	defer ev.Send()
	buildURL := fmt.Sprintf("%s/-/pipelines/%d", p.Project.WebURL, p.ObjectAttributes.ID)
	err = ev.Add(map[string]interface{}{
		// Basic trace information
		"service_name":   "pipeline",
		"trace.span_id":  traceID,
		"trace.trace_id": traceID,
		"name":           "build " + traceID,

		// CI information
		"ci_provider": "GitLab-CI",
		"branch":      p.ObjectAttributes.Ref,
		"build_num":   p.ObjectAttributes.ID,
		"build_url":   buildURL,
		"pr_number":   p.MergeRequest.Iid,
		"pr_branch":   p.MergeRequest.SourceBranch,
		// TODO: Replace project Id with SOURCE_PROJECT_PATH
		"pr_repo": p.MergeRequest.SourceProjectID,
		"repo":    p.Project.WebURL,
		// TODO: Something with pipeline status
		"status": p.ObjectAttributes.Status,

		"duration_ms":        p.ObjectAttributes.Duration * 1000,
		"queued_duration_ms": p.ObjectAttributes.QueuedDuration * 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add fields to event: %w", err)
	}

	if p.ObjectAttributes.CreatedAt.IsZero() {
		return nil, errors.New("Pipeline.ObjectAttributes.CreatedAt is zero")
	}
	ev.Timestamp = p.ObjectAttributes.CreatedAt
	log.Printf("%+v\n", ev)
	return ev, nil
}

func createTraceFromJob(cfg *libhoney.Config, j Job) (*libhoney.Event, error) {
	// if j.BuildStatus == "created" || j.BuildStatus == "running" || j.BuildStatus == "pending" {
	// 	return nil, nil
	// }
	if j.BuildDuration == 0 || j.BuildStatus == "running" {
		return nil, nil
	}
	parentTraceID := fmt.Sprint(j.PipelineID)
	md5HashInBytes := md5.Sum([]byte(j.BuildName))
	md5HashInString := hex.EncodeToString(md5HashInBytes[:])
	spanID := md5HashInString
	ev, err := createEvent(cfg)
	if err != nil {
		return nil, err
	}

	defer ev.Send()
	err = ev.Add(map[string]interface{}{
		// Basic trace information
		"service_name":    "job",
		"trace.span_id":   spanID,
		"trace.trace_id":  parentTraceID,
		"trace.parent_id": parentTraceID,
		"name":            fmt.Sprintf(j.BuildName),

		// CI information
		"ci_provider": "GitLab-CI",
		"branch":      j.Ref,
		"build_num":   j.PipelineID,
		"build_id":    j.BuildID,
		"repo":        j.Repository.Homepage,
		// TODO: Something with job status
		"status": j.BuildStatus,

		// Runner information
		"ci_runner":      j.Runner.Description,
		"ci_runner_id":   j.Runner.ID,
		"ci_runner_tags": strings.Join(j.Runner.Tags, ","),

		"duration_ms":        j.BuildDuration * 1000,
		"queued_duration_ms": j.BuildQueuedDuration * 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add fields to event: %w", err)
	}

	if j.BuildStartedAt.IsZero() {
		return nil, errors.New("BuildStartedAt time is not set")
	}
	ev.Timestamp = j.BuildStartedAt
	log.Printf("%+v\n", ev)
	return ev, nil
}

// buildevents build $CI_PIPELINE_ID $BUILD_START (failure|success)
func handlePipeline(cfg *libhoney.Config, w http.ResponseWriter, body []byte) {
	var pipeline Pipeline
	err := json.Unmarshal(body, &pipeline)
	if err != nil {
		log.Printf("Error unmarshalling request body: %s", err)
		_, printErr := fmt.Fprintf(w, "Error unmarshalling request body.")
		if printErr != nil {
			log.Print("Error printing error on error unmarshalling request body.")
		}
		return
	}
	_, err = createTraceFromPipeline(cfg, pipeline)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, respErr := fmt.Fprintf(w, "Error creating trace from pipeline object: %s", err)
		if respErr != nil {
			log.Printf("failed to write error response: %s", respErr)
		}
		return
	}

	_, respErr := fmt.Fprint(w, "Thanks!\n")
	if respErr != nil {
		log.Printf("failed to write success response: %s", respErr)
	}
}

// buildevents step $CI_PIPELINE_ID $STEP_SPAN_ID $STEP_START $CI_JOB_NAME
func handleJob(cfg *libhoney.Config, w http.ResponseWriter, body []byte) {
	var job Job
	err := json.Unmarshal(body, &job)
	if err != nil {
		log.Print("Error unmarshalling request body.")
		_, printErr := fmt.Fprintf(w, "Error unmarshalling request body.")
		if printErr != nil {
			log.Print("Error printing error on error unmarshalling request body.")
		}
		return
	}
	// fmt.Printf("%+v\n", job)
	_, err = createTraceFromJob(cfg, job)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, respErr := fmt.Fprintf(w, "Error creating trace from job object: %s", err)
		if respErr != nil {
			log.Printf("failed to write error response: %s", respErr)
		}
		return
	}

	_, respErr := fmt.Fprint(w, "Thanks!\n")
	if respErr != nil {
		log.Printf("failed to write success response: %s", respErr)
	}
}

func handleRequest(defaultConfig *libhoney.Config, w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}
	eventHeaders, exists := req.Header["X-Gitlab-Event"]
	if !exists {
		http.Error(w, "Missing header: X-Giitlab-Event", http.StatusBadRequest)
		return
	}

	if len(eventHeaders) > 1 {
		http.Error(w, "Invalid header: X-Gitlab-Event", http.StatusBadRequest)
		return
	}

	eventType := eventHeaders[0]
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Print("Error reading request body.")
		_, printErr := fmt.Fprintf(w, "Error reading request body.")
		if printErr != nil {
			log.Print("Error printing error on error reading request body.")
		}
		return
	}

	// Load potential custom query parameters
	// e.g. API key, dataset name, and honeycomb host
	// TODO: check for additional unsupported query parameters
	cfg := &libhoney.Config{}
	cfg.APIKey = defaultConfig.APIKey
	cfg.Dataset = defaultConfig.Dataset
	cfg.APIHost = defaultConfig.APIHost
	query := req.URL.Query()
	key := query.Get("api_key")
	if key != "" {
		cfg.APIKey = key
	}
	dataset := query.Get("dataset")
	if dataset != "" {
		cfg.Dataset = dataset
	}
	host := query.Get("api_host")
	if host != "" {
		cfg.APIHost = host
	}

	switch eventType {
	case "Pipeline Hook":
		log.Println("Received pipeline webhook:", string(body))
		handlePipeline(cfg, w, body)
	case "Job Hook":
		log.Println("Received job webhook:", string(body))
		handleJob(cfg, w, body)
	default:
		http.Error(w, fmt.Sprintf("Invalid event type: %s", eventType), http.StatusBadRequest)
	}
}

func commandRoot(cfg *libhoney.Config) *cobra.Command {
	root := &cobra.Command{
		Version: Version,
		Use:     "buildevents",
		Short:   "buildevents creates events for your CI builds",
		Long: `
The buildevents executable creates Honeycomb events and tracing information
about your Continuous Integration builds.`,
	}

	root.PersistentFlags().StringVarP(&cfg.APIKey, "apikey", "k", "", "[env.BUILDEVENT_APIKEY] the Honeycomb authentication token")
	if apikey, ok := os.LookupEnv("BUILDEVENT_APIKEY"); ok {
		// https://github.com/spf13/viper/issues/461#issuecomment-366831834
		err := root.PersistentFlags().Lookup("apikey").Value.Set(apikey)
		if err != nil {
			log.Fatalf("failed to configure `apikey`: %s", err)
		}
	}

	root.PersistentFlags().StringVarP(&cfg.Dataset, "dataset", "d", "buildevents", "[env.BUILDEVENT_DATASET] the name of the Honeycomb dataset to which to send these events")
	if dataset, ok := os.LookupEnv("BUILDEVENT_DATASET"); ok {
		err := root.PersistentFlags().Lookup("dataset").Value.Set(dataset)
		if err != nil {
			log.Fatalf("failed to configure `dataset`: %s", err)
		}
	}

	root.PersistentFlags().StringVarP(&cfg.APIHost, "apihost", "a", "https://api.honeycomb.io", "[env.BUILDEVENT_APIHOST] the hostname for the Honeycomb API server to which to send this event")
	if apihost, ok := os.LookupEnv("BUILDEVENT_APIHOST"); ok {
		err := root.PersistentFlags().Lookup("apihost").Value.Set(apihost)
		if err != nil {
			log.Fatalf("failed to configure `apihost`: %s", err)
		}
	}

	return root
}

func main() {
	defer libhoney.Close()
	var config libhoney.Config

	root := commandRoot(&config)

	// Do the work
	if err := root.Execute(); err != nil {
		libhoney.Close()
		os.Exit(1)
	}

	err := libhoney.Init(config)
	if err != nil {
		log.Fatalf("failed to initialise libhoney: %s", err)
	}

	log.SetOutput(os.Stdout)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/api/message", func(rw http.ResponseWriter, r *http.Request) {
		handleRequest(&config, rw, r)
	})
	mux.HandleFunc("/", home)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%s", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Printf("Starting server on http://%s\n", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    pipeline, err := UnmarshalPipeline(bytes)
//    bytes, err = pipeline.Marshal()

func UnmarshalPipeline(data []byte) (Pipeline, error) {
	var r Pipeline
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Pipeline) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Pipeline struct {
	ObjectKind       string                   `json:"object_kind"`
	ObjectAttributes PipelineObjectAttributes `json:"object_attributes"`
	MergeRequest     MergeRequest             `json:"merge_request"`
	User             User                     `json:"user"`
	Project          Project                  `json:"project"`
	Commit           Commit                   `json:"commit"`
	Builds           []Build                  `json:"builds"`
}

type Build struct {
	ID            int64         `json:"id"`
	Stage         string        `json:"stage"`
	Name          string        `json:"name"`
	Status        string        `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
	StartedAt     time.Time     `json:"started_at"`
	FinishedAt    time.Time     `json:"finished_at"`
	When          string        `json:"when"`
	Manual        bool          `json:"manual"`
	AllowFailure  bool          `json:"allow_failure"`
	User          User          `json:"user"`
	Runner        *Runner       `json:"runner"`
	ArtifactsFile ArtifactsFile `json:"artifacts_file"`
	Environment   *Environment  `json:"environment"`
}

type ArtifactsFile struct {
	Filename interface{} `json:"filename"`
	Size     interface{} `json:"size"`
}

type Environment struct {
	Name   string `json:"name"`
	Action string `json:"action"`
}

type Runner struct {
	Active      bool     `json:"active"`
	IsShared    bool     `json:"is_shared"`
	ID          int64    `json:"id"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type User struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

type Commit struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	URL       string `json:"url"`
	Author    Author `json:"author"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type MergeRequest struct {
	ID              int64  `json:"id"`
	Iid             int64  `json:"iid"`
	Title           string `json:"title"`
	SourceBranch    string `json:"source_branch"`
	SourceProjectID int64  `json:"source_project_id"`
	TargetBranch    string `json:"target_branch"`
	TargetProjectID int64  `json:"target_project_id"`
	State           string `json:"state"`
	MergeStatus     string `json:"merge_status"`
	URL             string `json:"url"`
}

type PipelineObjectAttributes struct {
	ID             int64      `json:"id"`
	Ref            string     `json:"ref"`
	Tag            bool       `json:"tag"`
	SHA            string     `json:"sha"`
	BeforeSHA      string     `json:"before_sha"`
	Source         string     `json:"source"`
	Status         string     `json:"status"`
	Stages         []string   `json:"stages"`
	CreatedAt      time.Time  `json:"created_at"`
	FinishedAt     time.Time  `json:"finished_at"`
	Duration       int64      `json:"duration"`
	QueuedDuration int64      `json:"queued_duration"`
	Variables      []Variable `json:"variables"`
}

type Variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Project struct {
	ID                int64       `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	WebURL            string      `json:"web_url"`
	AvatarURL         interface{} `json:"avatar_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	Namespace         string      `json:"namespace"`
	VisibilityLevel   int64       `json:"visibility_level"`
	PathWithNamespace string      `json:"path_with_namespace"`
	DefaultBranch     string      `json:"default_branch"`
}

// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    job, err := UnmarshalJob(bytes)
//    bytes, err = job.Marshal()

func UnmarshalJob(data []byte) (Job, error) {
	var r Job
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Job) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Job struct {
	ObjectKind          string      `json:"object_kind"`
	Ref                 string      `json:"ref"`
	Tag                 bool        `json:"tag"`
	BeforeSHA           string      `json:"before_sha"`
	SHA                 string      `json:"sha"`
	BuildID             int64       `json:"build_id"`
	BuildName           string      `json:"build_name"`
	BuildStage          string      `json:"build_stage"`
	BuildStatus         string      `json:"build_status"`
	BuildCreatedAt      time.Time   `json:"build_created_at"`
	BuildStartedAt      time.Time   `json:"build_started_at"`
	BuildFinishedAt     time.Time   `json:"build_finished_at"`
	BuildDuration       float64     `json:"build_duration"`
	BuildQueuedDuration float64     `json:"build_queued_duration"`
	BuildAllowFailure   bool        `json:"build_allow_failure"`
	BuildFailureReason  string      `json:"build_failure_reason"`
	PipelineID          int64       `json:"pipeline_id"`
	ProjectID           int64       `json:"project_id"`
	ProjectName         string      `json:"project_name"`
	User                User        `json:"user"`
	Commit              JobCommit   `json:"commit"`
	Repository          Repository  `json:"repository"`
	Runner              Runner      `json:"runner"`
	Environment         interface{} `json:"environment"`
}

type JobCommit struct {
	ID          int64       `json:"id"`
	SHA         string      `json:"sha"`
	Message     string      `json:"message"`
	AuthorName  string      `json:"author_name"`
	AuthorEmail string      `json:"author_email"`
	Status      string      `json:"status"`
	Duration    interface{} `json:"duration"`
	StartedAt   interface{} `json:"started_at"`
	FinishedAt  interface{} `json:"finished_at"`
}

type Repository struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Homepage        string `json:"homepage"`
	GitSSHURL       string `json:"git_ssh_url"`
	GitHTTPURL      string `json:"git_http_url"`
	VisibilityLevel int64  `json:"visibility_level"`
}
