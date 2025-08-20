package merics

import (
	"time"

	"golang.org/x/net/context"
)

type Endpoint struct {
}

func (e *Endpoint) PingHandler(ctx context.Context, input *struct{}) (*struct {
	Body PingResponse `json:"body"`
}, error) {
	return &struct {
		Body PingResponse `json:"body"`
	}{
		Body: PingResponse{Time: time.Now().Format(time.RFC3339)},
	}, nil
}
