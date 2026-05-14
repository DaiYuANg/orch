//go:build !linux

package firecracker

type runningState struct{}

func newRunningState() runningState {
	return runningState{}
}
