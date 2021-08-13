package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/honeycombio/libhoney-go"
	"github.com/honeycombio/libhoney-go/transmission"
	"github.com/spf13/cobra"
)

// This is the default value that should be overridden in the
// build/release process.
var Version = "dev"

func home(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, `# GitLab Honeycomb Buildevents Webhooks Sink

GET /healthz: healthcheck

POST /api/message: receive array of notifications
`)
}

func healthz(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "OK\n")
}

func createEvent(cfg *libhoney.Config, traceID string) *libhoney.Event {
	libhoney.UserAgentAddition = fmt.Sprintf("buildevents/%s", Version)
	libhoney.UserAgentAddition += fmt.Sprintf(" (%s)", "GitLab-CI")

	if cfg.APIKey == "" {
		cfg.Transmission = &transmission.WriterSender{}
	}
	libhoney.Init(*cfg)

	ev := libhoney.NewEvent()
	ev.AddField("ci_provider", "GitLab-CI")
	ev.AddField("trace.trace_id", traceID)
	ev.AddField("meta.version", Version)

	return ev
}

func createTraceFromPipeline(cfg *libhoney.Config, p Pipeline) {
	ev := createEvent(cfg, fmt.Sprint(p.ObjectAttributes.ID))
	ev.AddField("ci_provider", "GitLab-CI")
	// ev.AddField("trace.trace_id", p.ObjectAttributes.ID)
	ev.AddField("branch", p.ObjectAttributes.Ref)
	ev.AddField("build_num", p.ObjectAttributes.ID)
	buildURL := fmt.Sprintf("%s/-/pipelines/%d", p.Project.WebURL, p.ObjectAttributes.ID)
	ev.AddField("build_url", buildURL)
	ev.AddField("pr_number", p.MergeRequest.Iid)
	ev.AddField("pr_branch", p.MergeRequest.SourceBranch)

	// TODO: Replace project Id with SOURCE_PROJECT_PATH
	ev.AddField("pr_repo", p.MergeRequest.SourceProjectID)
	// "CI_MERGE_REQUEST_SOURCE_PROJECT_PATH": "pr_repo",

	ev.AddField("repo", p.Project.WebURL)

	// TODO: Something with pipeline status
	ev.AddField("status", p.ObjectAttributes.Status)
	fmt.Printf("%+v\n", ev)
}

func createTraceFromJob(cfg *libhoney.Config, j Job) {
	ev := createEvent(cfg, fmt.Sprint(j.PipelineID))
	ev.AddField("ci_provider", "GitLab-CI")
	// ev.AddField("trace.trace_id", j.ObjectAttributes.ID)
	ev.AddField("branch", j.Ref)
	ev.AddField("build_num", j.PipelineID)
	// buildURL := fmt.Sprintf("%s/-/pipelines/%d", j.Project.WebURL, j.ObjectAttributes.ID)
	// ev.AddField("build_url", buildURL)
	// ev.AddField("pr_number", j.MergeRequest.Iid)
	// ev.AddField("pr_branch", j.MergeRequest.SourceBranch)

	// TODO: Replace project Id with SOURCE_PROJECT_PATH
	// ev.AddField("pr_repo", j.MergeRequest.SourceProjectID)
	// "CI_MERGE_REQUEST_SOURCE_PROJECT_PATH": "pr_repo",

	ev.AddField("repo", j.Repository.Homepage)

	// TODO: Something with pipeline status
	ev.AddField("status", j.BuildStatus)
	fmt.Printf("%+v\n", ev)
}

// buildevents build $CI_PIPELINE_ID $BUILD_START (failure|success)
func handlePipeline(cfg *libhoney.Config, w http.ResponseWriter, body []byte) {
	var pipeline Pipeline
	err := json.Unmarshal(body, &pipeline)
	if err != nil {
		log.Print("Error unmarshalling request body.")
		_, printErr := fmt.Fprintf(w, "Error unmarshalling request body.")
		if printErr != nil {
			log.Print("Error printing error on error unmarshalling request body.")
		}
		return
	}
	createTraceFromPipeline(cfg, pipeline)
	fmt.Fprintf(w, "Thanks!\n")
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
	createTraceFromJob(cfg, job)
	fmt.Fprintf(w, "Thanks!\n")
}

func hello(cfg *libhoney.Config, w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}
	eventHeaders := req.Header["X-Gitlab-Event"]
	if len(eventHeaders) < 1 {
		http.Error(w, "Missing header: X-Giitlab-Event", http.StatusBadRequest)
		return
	} else if len(eventHeaders) > 1 {
		http.Error(w, "Invalid header: X-Gitlab-Event", http.StatusBadRequest)
		return
	}
	eventType := eventHeaders[0]
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Print("Error reading request body.")
		_, printErr := fmt.Fprintf(w, "Error reading request body.")
		if printErr != nil {
			log.Print("Error printing error on error reading request body.")
		}
		return
	}
	if eventType == "Pipeline Hook" {
		handlePipeline(cfg, w, body)
	} else if eventType == "Job Hook" {
		handleJob(cfg, w, body)
	} else {
		http.Error(w, fmt.Sprintf("Invalid event type: %s", eventType), http.StatusBadRequest)
		return
	}
}

func commandRoot(cfg *libhoney.Config, filename *string, ciProvider *string) *cobra.Command {
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
		root.PersistentFlags().Lookup("apikey").Value.Set(apikey)
	}

	root.PersistentFlags().StringVarP(&cfg.Dataset, "dataset", "d", "buildevents", "[env.BUILDEVENT_DATASET] the name of the Honeycomb dataset to which to send these events")
	if dataset, ok := os.LookupEnv("BUILDEVENT_DATASET"); ok {
		root.PersistentFlags().Lookup("dataset").Value.Set(dataset)
	}

	root.PersistentFlags().StringVarP(&cfg.APIHost, "apihost", "a", "https://api.honeycomb.io", "[env.BUILDEVENT_APIHOST] the hostname for the Honeycomb API server to which to send this event")
	if apihost, ok := os.LookupEnv("BUILDEVENT_APIHOST"); ok {
		root.PersistentFlags().Lookup("apihost").Value.Set(apihost)
	}

	root.PersistentFlags().StringVarP(filename, "filename", "f", "", "[env.BUILDEVENT_FILE] the path of a text file holding arbitrary key=val pairs (multi-line-capable, logfmt style) to be added to the Honeycomb event")
	if fname, ok := os.LookupEnv("BUILDEVENT_FILE"); ok {
		root.PersistentFlags().Lookup("filename").Value.Set(fname)
	}

	root.PersistentFlags().StringVarP(ciProvider, "provider", "p", "GitLab-CI", "[env.BUILDEVENT_CIPROVIDER] if unset, will inspect the environment to try to detect common CI providers.")

	return root
}

func main() {
	defer libhoney.Close()
	var config libhoney.Config
	var filename string
	var ciProvider string
	// var wcfg watchConfig

	root := commandRoot(&config, &filename, &ciProvider)

	// Put 'em all together
	root.AddCommand(
	// commandBuild(&config, &filename, &ciProvider),
	// commandStep(&config, &filename, &ciProvider),
	// commandCmd(&config, &filename, &ciProvider),
	// commandWatch(&config, &filename, &ciProvider, &wcfg),
	)

	// Do the work
	if err := root.Execute(); err != nil {
		libhoney.Close()
		os.Exit(1)
	}
	log.SetOutput(os.Stdout)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/api/message", func(rw http.ResponseWriter, r *http.Request) {
		hello(&config, rw, r)
	})
	mux.HandleFunc("/", home)
	srv := &http.Server{
		Addr:         "0.0.0.0:8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	fmt.Printf("Starting server on http://%s\n", srv.Addr)
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
	ObjectKind       string           `json:"object_kind"`
	ObjectAttributes ObjectAttributes `json:"object_attributes"`
	MergeRequest     MergeRequest     `json:"merge_request"`
	User             User             `json:"user"`
	Project          Project          `json:"project"`
	Commit           Commit           `json:"commit"`
	Builds           []Build          `json:"builds"`
}

type Build struct {
	ID            int64         `json:"id"`
	Stage         string        `json:"stage"`
	Name          string        `json:"name"`
	Status        string        `json:"status"`
	CreatedAt     string        `json:"created_at"`
	StartedAt     *string       `json:"started_at"`
	FinishedAt    *string       `json:"finished_at"`
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

type ObjectAttributes struct {
	ID         int64      `json:"id"`
	Ref        string     `json:"ref"`
	Tag        bool       `json:"tag"`
	SHA        string     `json:"sha"`
	BeforeSHA  string     `json:"before_sha"`
	Source     string     `json:"source"`
	Status     string     `json:"status"`
	Stages     []string   `json:"stages"`
	CreatedAt  string     `json:"created_at"`
	FinishedAt string     `json:"finished_at"`
	Duration   int64      `json:"duration"`
	Variables  []Variable `json:"variables"`
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
	ObjectKind         string      `json:"object_kind"`
	Ref                string      `json:"ref"`
	Tag                bool        `json:"tag"`
	BeforeSHA          string      `json:"before_sha"`
	SHA                string      `json:"sha"`
	BuildID            int64       `json:"build_id"`
	BuildName          string      `json:"build_name"`
	BuildStage         string      `json:"build_stage"`
	BuildStatus        string      `json:"build_status"`
	BuildCreatedAt     string      `json:"build_created_at"`
	BuildStartedAt     interface{} `json:"build_started_at"`
	BuildFinishedAt    interface{} `json:"build_finished_at"`
	BuildDuration      interface{} `json:"build_duration"`
	BuildAllowFailure  bool        `json:"build_allow_failure"`
	BuildFailureReason string      `json:"build_failure_reason"`
	PipelineID         int64       `json:"pipeline_id"`
	ProjectID          int64       `json:"project_id"`
	ProjectName        string      `json:"project_name"`
	User               User        `json:"user"`
	Commit             JobCommit   `json:"commit"`
	Repository         Repository  `json:"repository"`
	Runner             Runner      `json:"runner"`
	Environment        interface{} `json:"environment"`
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
