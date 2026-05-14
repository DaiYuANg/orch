package cmd

import (
	"strconv"

	"github.com/arcgolabs/collectionx/list"

	"github.com/lyonbrown4d/orch/internal/api"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func writeReadyHuman(out *api.ReadyOutput) error {
	if out == nil {
		return writeLine(viewMutedStyle.Render("No readiness response."))
	}
	if err := writeInfoLine("ready",
		viewField("status", statusBadge(out.Body.Status)),
		viewField("time", out.Body.Timestamp),
	); err != nil {
		return err
	}
	rows := list.NewGridWithCapacity[string](out.Body.Checks.Len())
	out.Body.Checks.Range(func(_ int, check api.ReadyCheckItem) bool {
		rows.AddRow(check.Name, strconv.FormatBool(check.Ready), statusBadge(check.Status), nonEmpty(check.Detail))
		return true
	})
	return writeTable(list.NewList("CHECK", "READY", "STATUS", "DETAIL"), rows)
}

func writeEventsHuman(items []api.AssignmentItem) error {
	rows := list.NewGridWithCapacity[string](len(items))
	for i := range items {
		item := &items[i]
		rows.AddRow(formatTime(item.UpdatedAt), item.Key, nonEmpty(item.Node), string(item.Runtime), statusBadge(item.Status), nonEmpty(item.Error))
	}
	return writeTable(list.NewList("TIME", "KEY", "NODE", "RUNTIME", "STATUS", "ERROR"), rows)
}

func writeLocalNodeHuman(name string, raft *api.RaftStatusOutput, host *api.HostinfoOutput) error {
	if raft == nil || host == nil {
		return writeLine(viewMutedStyle.Render("No node response."))
	}
	body := host.Body
	rows := list.NewGrid[string](
		[]string{"node_id", nonEmpty(name)},
		[]string{"hostname", nonEmpty(body.Host.Hostname)},
		[]string{"os", body.Host.OS},
		[]string{"arch", body.Host.KernelArch},
		[]string{"raft_ready", strconv.FormatBool(raft.Body.Ready)},
		[]string{"raft_state", statusBadge(raft.Body.State)},
		[]string{"raft_leader", nonEmpty(raft.Body.LeaderID)},
		[]string{"raft_address", nonEmpty(raft.Body.LocalAddress)},
		[]string{"cpu_cores", strconv.Itoa(body.CPU.LogicalCores)},
		[]string{"cpu_usage", formatPercent(body.CPU.UsagePercent)},
		[]string{"memory_total", formatBytes(body.Memory.TotalBytes)},
		[]string{"memory_used", formatPercent(body.Memory.UsedPercent)},
	)
	return writeKVTable(rows)
}

func writeRaftMemberNodeHuman(name string, raft *api.RaftStatusOutput) error {
	if raft == nil || raft.Body.Members == nil {
		return writeLine(viewMutedStyle.Render("No node response."))
	}
	var found api.RaftMemberItem
	ok := false
	raft.Body.Members.Range(func(_ int, member api.RaftMemberItem) bool {
		if member.ID != name {
			return true
		}
		found = member
		ok = true
		return false
	})
	if !ok {
		return oopsx.B("cli").Errorf("node %q not found in raft members", name)
	}
	rows := list.NewGrid[string](
		[]string{"node_id", found.ID},
		[]string{"raft_address", found.Address},
		[]string{"suffrage", found.Suffrage},
		[]string{"leader", strconv.FormatBool(found.ID == raft.Body.LeaderID)},
		[]string{"local", strconv.FormatBool(found.ID == raft.Body.NodeID)},
	)
	return writeKVTable(rows)
}

func writeWorkloadRuntimeStatusHuman(out *api.WorkloadRuntimeStatusOutput) error {
	if out == nil {
		return writeLine(viewMutedStyle.Render("No workload status response."))
	}
	body := out.Body
	rows := list.NewGrid[string](
		[]string{"name", nonEmpty(body.Name)},
		[]string{"runtime", string(body.Runtime)},
		[]string{"status", statusBadge(body.Status)},
		[]string{"native_id", nonEmpty(body.NativeID)},
		[]string{"started_at", formatTime(body.StartedAt)},
		[]string{"updated_at", formatTime(body.UpdatedAt)},
		[]string{"message", nonEmpty(body.Message)},
	)
	return writeKVTable(rows)
}
