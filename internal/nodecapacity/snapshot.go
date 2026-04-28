package nodecapacity

import (
	"time"

	"github.com/daiyuang/orch/internal/hostinfo"
)

// Snapshot is a lightweight node resource view used for placement.
type Snapshot struct {
	NodeID           string    `json:"nodeId"`
	UpdatedAt        time.Time `json:"updatedAt"`
	LogicalCPUCores  int       `json:"logicalCpuCores"`
	CPUUsagePercent  float64   `json:"cpuUsagePercent"`
	MemoryTotalBytes uint64    `json:"memoryTotalBytes"`
	MemoryAvailBytes uint64    `json:"memoryAvailableBytes"`
	MemoryUsedPct    float64   `json:"memoryUsedPercent"`
}

func snapshotFromReport(nodeID string, rep *hostinfo.Report) Snapshot {
	s := Snapshot{
		NodeID:          nodeID,
		UpdatedAt:       time.Now().UTC(),
		LogicalCPUCores: rep.CPU.LogicalCores,
		CPUUsagePercent: rep.CPU.UsagePercent,
	}
	s.MemoryTotalBytes = rep.Memory.TotalBytes
	s.MemoryAvailBytes = rep.Memory.AvailableBytes
	s.MemoryUsedPct = rep.Memory.UsedPercent
	return s
}
