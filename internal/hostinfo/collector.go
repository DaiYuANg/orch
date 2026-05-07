package hostinfo

import (
	"context"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// Report is a JSON-friendly snapshot of the local machine.
type Report struct {
	Host   HostSection   `json:"host"`
	CPU    CPUSection    `json:"cpu"`
	Memory MemorySection `json:"memory"`
	Load   *LoadSection  `json:"load,omitempty"`
	Disks  []DiskEntry   `json:"disks,omitempty"`
}

type HostSection struct {
	Hostname       string `json:"hostname,omitempty"`
	UptimeSeconds  uint64 `json:"uptimeSeconds,omitempty"`
	BootTimeUnix   uint64 `json:"bootTimeUnix,omitempty"`
	OS             string `json:"os,omitempty"`
	Platform       string `json:"platform,omitempty"`
	PlatformFamily string `json:"platformFamily,omitempty"`
	KernelVersion  string `json:"kernelVersion,omitempty"`
	KernelArch     string `json:"kernelArch,omitempty"`
	Virtualization string `json:"virtualization,omitempty"`
}

type CPUSection struct {
	LogicalCores int     `json:"logicalCores,omitempty"`
	ModelName    string  `json:"modelName,omitempty"`
	UsagePercent float64 `json:"usagePercent,omitempty"`
}

type MemorySection struct {
	TotalBytes     uint64  `json:"totalBytes,omitempty"`
	AvailableBytes uint64  `json:"availableBytes,omitempty"`
	UsedBytes      uint64  `json:"usedBytes,omitempty"`
	UsedPercent    float64 `json:"usedPercent,omitempty"`
}

type LoadSection struct {
	Load1  float64 `json:"load1,omitempty"`
	Load5  float64 `json:"load5,omitempty"`
	Load15 float64 `json:"load15,omitempty"`
}

type DiskEntry struct {
	Device      string  `json:"device,omitempty"`
	Mountpoint  string  `json:"mountpoint,omitempty"`
	Fstype      string  `json:"fstype,omitempty"`
	TotalBytes  uint64  `json:"totalBytes,omitempty"`
	FreeBytes   uint64  `json:"freeBytes,omitempty"`
	UsedBytes   uint64  `json:"usedBytes,omitempty"`
	UsedPercent float64 `json:"usedPercent,omitempty"`
}

const cpuSampleWait = 200 * time.Millisecond
const maxDiskPartitions = 24

func collectHostSection(ctx context.Context, out *Report) {
	if hi, err := host.InfoWithContext(ctx); err == nil && hi != nil {
		out.Host = HostSection{
			Hostname:       hi.Hostname,
			UptimeSeconds:  hi.Uptime,
			BootTimeUnix:   hi.BootTime,
			OS:             hi.OS,
			Platform:       hi.Platform,
			PlatformFamily: hi.PlatformFamily,
			KernelVersion:  hi.KernelVersion,
			KernelArch:     hi.KernelArch,
			Virtualization: hi.VirtualizationSystem,
		}
	}
}

func collectCPUSection(ctx context.Context, out *Report) {
	if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
		out.CPU.ModelName = infos[0].ModelName
	}
	if n, err := cpu.CountsWithContext(ctx, true); err == nil {
		out.CPU.LogicalCores = n
	}
	if pct, err := cpu.PercentWithContext(ctx, cpuSampleWait, false); err == nil && len(pct) > 0 {
		out.CPU.UsagePercent = pct[0]
	}
}

func collectMemorySection(ctx context.Context, out *Report) {
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil && vm != nil {
		out.Memory = MemorySection{
			TotalBytes:     vm.Total,
			AvailableBytes: vm.Available,
			UsedBytes:      vm.Used,
			UsedPercent:    vm.UsedPercent,
		}
	}
}

func collectLoadSection(ctx context.Context, out *Report) {
	if avg, err := load.AvgWithContext(ctx); err == nil && avg != nil {
		out.Load = &LoadSection{
			Load1:  avg.Load1,
			Load5:  avg.Load5,
			Load15: avg.Load15,
		}
	}
}

func collectDiskSection(ctx context.Context, out *Report) {
	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return
	}
	out.Disks = buildDiskEntries(ctx, parts)
}

func buildDiskEntries(ctx context.Context, parts []disk.PartitionStat) []DiskEntry {
	return buildDiskEntryList(ctx, parts).Values()
}

func buildDiskEntryList(ctx context.Context, parts []disk.PartitionStat) *list.List[DiskEntry] {
	seen := set.NewSet[string]()
	disks := list.NewListWithCapacity[DiskEntry](maxDiskPartitions)
	for _, p := range parts {
		if disks.Len() >= maxDiskPartitions {
			break
		}
		mp := p.Mountpoint
		if mp == "" || seen.Contains(mp) {
			continue
		}
		seen.Add(mp)
		usage, err := disk.UsageWithContext(ctx, mp)
		if err != nil || usage == nil {
			continue
		}
		disks.Add(DiskEntry{
			Device:      p.Device,
			Mountpoint:  mp,
			Fstype:      p.Fstype,
			TotalBytes:  usage.Total,
			FreeBytes:   usage.Free,
			UsedBytes:   usage.Used,
			UsedPercent: usage.UsedPercent,
		})
	}
	disks.Sort(func(a, b DiskEntry) int {
		return strings.Compare(a.Mountpoint, b.Mountpoint)
	})
	return disks
}

// Collect gathers host statistics. CPU usage uses a short blocking sample (~cpuSampleWait).
func Collect(ctx context.Context) (*Report, error) {
	out := &Report{}
	collectHostSection(ctx, out)
	collectCPUSection(ctx, out)
	collectMemorySection(ctx, out)
	collectLoadSection(ctx, out)
	collectDiskSection(ctx, out)
	return out, nil
}
