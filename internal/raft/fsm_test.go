package raft

import (
	"io"
	"log/slog"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSMApplySetReadDelete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fsm, err := newFsm(t.TempDir(), logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = fsm.close()
	})

	setPayload, err := encodeCommand(newSetCommand("bucket-a", "key-a", []byte(`{"ok":true}`)))
	require.NoError(t, err)

	result := fsm.Apply(&raft.Log{Data: setPayload})
	require.Nil(t, result)

	value, err := fsm.Read("bucket-a", "key-a")
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(value))

	deletePayload, err := encodeCommand(newDeleteCommand("bucket-a", "key-a"))
	require.NoError(t, err)

	result = fsm.Apply(&raft.Log{Data: deletePayload})
	require.Nil(t, result)

	_, err = fsm.Read("bucket-a", "key-a")
	require.Error(t, err)
}
