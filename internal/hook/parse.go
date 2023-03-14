package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/zoidbergwill/gitlab-honeycomb-buildevents-webhooks-sink/internal/hook/types"
)

var (
	ErrInvalidHTTPMethod             = errors.New("invalid HTTP Method")
	ErrGitLabTokenVerificationFailed = errors.New("X-Gitlab-Token validation failed")
)

const (
	PipelineEvents = "Pipeline Hook"
	JobEvents      = "Job Hook"
)

type ErrPayloadParse struct {
	Payload []byte
	Err     error
}

func (epp ErrPayloadParse) Error() string {
	return epp.Err.Error()
}

func (epp ErrPayloadParse) Unwrap() error {
	return epp.Err
}

func (l *Listener) ParseHook(r *http.Request, event string) (interface{}, error) {
	if r.Method != http.MethodPost {
		return nil, ErrInvalidHTTPMethod
	}

	signature := r.Header.Get("X-Gitlab-Token")
	if signature != l.Config.HookSecret {
		return nil, ErrGitLabTokenVerificationFailed
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		return nil, ErrPayloadParse{Payload: payload, Err: err}
	}

	if l.Config.Debug {
		log.Printf("raw payload: %s", string(payload))
	}

	switch event {
	case PipelineEvents:
		var pe types.PipelineEventPayload
		err = json.Unmarshal(payload, &pe)
		if err != nil {
			return nil, fmt.Errorf("failed to parse payload into pipeline event: %w", err)
		}

		if l.Config.Debug {
			log.Printf("parsed pipeline event: %+v", pe)
		}

		return pe, nil
	case JobEvents:
		var je types.JobEventPayload
		err = json.Unmarshal(payload, &je)
		if err != nil {
			return nil, fmt.Errorf("failed to parse payload into job event: %w", err)
		}

		if l.Config.Debug {
			log.Printf("parsed job event: %+v", je)
		}

		return je, nil
	default:
		return nil, fmt.Errorf("%s is not a valid event we're catching", event)
	}
}
