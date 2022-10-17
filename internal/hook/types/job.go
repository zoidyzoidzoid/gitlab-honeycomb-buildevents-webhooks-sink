package types

// JobEventPayload contains the information for GitLab's Job status change
type JobEventPayload struct {
	ObjectKind         string          `json:"object_kind"`
	Ref                string          `json:"ref"`
	Tag                bool            `json:"tag"`
	BeforeSHA          string          `json:"before_sha"`
	SHA                string          `json:"sha"`
	BuildID            int64           `json:"build_id"`
	BuildName          string          `json:"build_name"`
	BuildStage         string          `json:"build_stage"`
	BuildStatus        string          `json:"build_status"`
	BuildStartedAt     GitLabTimestamp `json:"build_started_at,omitempty"`
	BuildFinishedAt    GitLabTimestamp `json:"build_finished_at,omitempty"`
	BuildDuration      float64         `json:"build_duration"`
	BuildAllowFailure  bool            `json:"build_allow_failure"`
	BuildFailureReason string          `json:"build_failure_reason"`
	PipelineID         int64           `json:"pipeline_id"`
	ProjectID          int64           `json:"project_id"`
	ProjectName        string          `json:"project_name"`
	User               User            `json:"user"`
	Commit             BuildCommit     `json:"commit"`
	Repository         Repository      `json:"repository"`
	Runner             Runner          `json:"runner"`
}

// BuildCommit contains all of the GitLab build commit information
type BuildCommit struct {
	ID          int64           `json:"id"`
	SHA         string          `json:"sha"`
	Message     string          `json:"message"`
	AuthorName  string          `json:"author_name"`
	AuthorEmail string          `json:"author_email"`
	Status      string          `json:"status"`
	Duration    float64         `json:"duration"`
	StartedAt   GitLabTimestamp `json:"started_at,omitempty"`
	FinishedAt  GitLabTimestamp `json:"finished_at,omitempty"`
}
