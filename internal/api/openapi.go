package api

import (
	"github.com/arcgolabs/httpx"
	"github.com/danielgtaylor/huma/v2"
)

// OpenAPIMeta sets Tags, OperationID, and Summary on the Huma operation (OpenAPI).
func OpenAPIMeta(tags []string, operationID, summary string) httpx.OperationOption {
	return func(op *huma.Operation) {
		op.Tags = append([]string(nil), tags...)
		op.OperationID = operationID
		op.Summary = summary
	}
}
