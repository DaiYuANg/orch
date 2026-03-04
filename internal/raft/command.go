package raft

import (
	"fmt"
	"strings"

	"github.com/goccy/go-json"
)

type commandType string

const (
	commandTypeSet    commandType = "set"
	commandTypeDelete commandType = "delete"
)

type command struct {
	Type   commandType `json:"type"`
	Bucket string      `json:"bucket"`
	Key    string      `json:"key"`
	Value  []byte      `json:"value,omitempty"`
}

func newSetCommand(bucket, key string, value []byte) command {
	return command{
		Type:   commandTypeSet,
		Bucket: bucket,
		Key:    key,
		Value:  value,
	}
}

func newDeleteCommand(bucket, key string) command {
	return command{
		Type:   commandTypeDelete,
		Bucket: bucket,
		Key:    key,
	}
}

func (c command) validate() error {
	if c.Type != commandTypeSet && c.Type != commandTypeDelete {
		return fmt.Errorf("unsupported command type: %s", c.Type)
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return fmt.Errorf("command bucket is required")
	}
	if strings.TrimSpace(c.Key) == "" {
		return fmt.Errorf("command key is required")
	}
	if c.Type == commandTypeSet && len(c.Value) == 0 {
		return fmt.Errorf("command value is required for set")
	}
	return nil
}

func encodeCommand(cmd command) ([]byte, error) {
	if err := cmd.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(cmd)
}

func decodeCommand(raw []byte) (command, error) {
	var cmd command
	if err := json.Unmarshal(raw, &cmd); err != nil {
		return command{}, err
	}
	return cmd, cmd.validate()
}

func cacheKey(bucket, key string) []byte {
	return []byte(bucket + ":" + key)
}
