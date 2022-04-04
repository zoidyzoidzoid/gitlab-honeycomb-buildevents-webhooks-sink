package hook

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Deichindianer/webhooks/v6/gitlab"
	"github.com/honeycombio/libhoney-go"
	"github.com/honeycombio/libhoney-go/transmission"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Listener struct {
	Config     Config
	Hook       *gitlab.Webhook
	HTTPServer *http.Server
}

type Config struct {
	Version         string
	ListenAddr      string
	HookSecret      string
	HoneycombConfig *libhoney.Config
}

type Honeycomb struct {
	Config *libhoney.Config
}

func New(cfg Config) (*Listener, error) {
	err := libhoney.Init(*cfg.HoneycombConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise libhoney: %w", err)
	}

	hook, err := gitlab.New(gitlab.Options.Secret(cfg.HookSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to setup GitLab webhook: %w", err)
	}

	l := Listener{
		Config: cfg,
		Hook:   hook,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", l.Healthz)
	mux.HandleFunc("/api/message", l.HandleRequest)
	mux.HandleFunc("/", l.Home)

	srv := &http.Server{
		Addr:         fmt.Sprintf(cfg.ListenAddr),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return &Listener{
		Config:     cfg,
		Hook:       hook,
		HTTPServer: srv,
	}, nil
}

func (l *Listener) Home(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

	_, err := fmt.Fprintf(w, `# GitLab Honeycomb Buildevents Webhooks Sink

GET /healthz: healthcheck

POST /api/message: receive array of notifications
`)
	if err != nil {
		log.Printf("home: failed to write to http response writer: %s", err)
	}
}

func (l *Listener) Healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusOK)
}

func (l *Listener) HandleRequest(w http.ResponseWriter, r *http.Request) {
	event, err := l.Hook.Parse(r, gitlab.PipelineEvents, gitlab.JobEvents)
	if err != nil {
		if errors.Is(err, gitlab.ErrParsingPayload) {
			log.Printf("failed to parse payload, dumping received payload: %+v", event)
			w.WriteHeader(http.StatusInternalServerError)
		}
		log.Printf("death: %s: %+v", err, event)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch e := event.(type) {
	case gitlab.PipelineEventPayload:
		err := l.handlePipeline(e)
		if err != nil {
			log.Printf("failed to handle pipeline event: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case gitlab.JobEventPayload:
		err := l.handleJob(e)
		if err != nil {
			log.Printf("failed to handle pipeline event: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, fmt.Sprintf("Invalid event type: %T", e), http.StatusBadRequest)
		return
	}

	_, respErr := fmt.Fprint(w, "Thanks!\n")
	if respErr != nil {
		log.Printf("failed to write success response: %s", respErr)
	}
}

func (l *Listener) handlePipeline(p gitlab.PipelineEventPayload) error {
	if p.ObjectAttributes.Duration == 0 || p.ObjectAttributes.Status == "running" {
		return nil
	}

	traceID := strconv.Itoa(int(p.ObjectAttributes.ID))
	ev, err := l.createEvent()
	if err != nil {
		return err
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
		"pr_number":   p.MergeRequest.IID,
		"pr_branch":   p.MergeRequest.SourceBranch,
		// TODO: Replace project Id with SOURCE_PROJECT_PATH
		"pr_repo": p.MergeRequest.SourceProjectID,
		"repo":    p.Project.WebURL,
		// TODO: Something with pipeline status
		"status": p.ObjectAttributes.Status,

		"duration_ms": p.ObjectAttributes.Duration * 1000,
	})
	if err != nil {
		return fmt.Errorf("failed to add fields to event: %w", err)
	}

	if p.ObjectAttributes.CreatedAt.IsZero() {
		return errors.New("Pipeline.ObjectAttributes.CreatedAt is zero")
	}
	ev.Timestamp = p.ObjectAttributes.CreatedAt.Time
	log.Printf("%+v\n", ev)
	return nil
}

func (l *Listener) handleJob(j gitlab.JobEventPayload) error {
	// if j.BuildStatus == "created" || j.BuildStatus == "running" || j.BuildStatus == "pending" {
	// 	return nil
	// }
	if j.BuildDuration == 0 || j.BuildStatus == "running" {
		return nil
	}
	parentTraceID := fmt.Sprint(j.PipelineID)
	md5HashInBytes := md5.Sum([]byte(j.BuildName))
	md5HashInString := hex.EncodeToString(md5HashInBytes[:])
	spanID := md5HashInString
	ev, err := l.createEvent()
	if err != nil {
		return err
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
		"ci_runner":    j.Runner.Description,
		"ci_runner_id": j.Runner.ID,
		//"ci_runner_tags": strings.Join(j.Runner.Tags, ","),

		"duration_ms": j.BuildDuration * 1000,
	})
	if err != nil {
		return fmt.Errorf("failed to add fields to event: %w", err)
	}

	if j.BuildStartedAt.IsZero() {
		return errors.New("BuildStartedAt time is not set")
	}

	ev.Timestamp = j.BuildStartedAt.Time
	return nil
}

func (l *Listener) createEvent() (*libhoney.Event, error) {
	libhoney.UserAgentAddition = fmt.Sprintf("buildevents/%s", l.Config.Version)
	libhoney.UserAgentAddition += fmt.Sprintf(" (%s)", "GitLab-CI")

	if l.Config.HoneycombConfig.APIKey == "" {
		l.Config.HoneycombConfig.Transmission = &transmission.WriterSender{}
	}

	ev := libhoney.NewEvent()
	ev.AddField("ci_provider", "GitLab-CI")
	ev.AddField("meta.version", l.Config.Version)

	return ev, nil
}

func (l *Listener) ListenAndServe() error {
	return l.HTTPServer.ListenAndServe()
}
