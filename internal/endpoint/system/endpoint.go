package system

import (
	"github.com/DaiYuANg/warden/internal/http/model"
	"github.com/DaiYuANg/warden/internal/raft"
	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"golang.org/x/net/context"
)

type Endpoint struct {
	raft *raft.Service
}

// CPUHandler
func (s *Endpoint) CPUHandler(ctx context.Context, input *struct{}) (*struct {
	Body model.Response[CPUResponse]
}, error) {
	ci, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	p, err := cpu.PercentWithContext(ctx, 0, true)

	if err != nil {
		return nil, err
	}

	resp := CPUResponse{}
	if len(ci) > 0 {
		resp.ModelName = ci[0].ModelName
		resp.Cores = ci[0].Cores
	}
	resp.Percent = p
	return model.WrapResponse(resp), nil
}

// MemHandler
func (s *Endpoint) MemHandler(ctx context.Context, input *struct{}) (*struct {
	Body model.Response[MemResponse]
}, error) {
	m, err := mem.VirtualMemoryWithContext(ctx)

	if err != nil {
		return nil, err
	}
	resp := MemResponse{
		Total:       m.Total,
		Used:        m.Used,
		Free:        m.Free,
		UsedPercent: m.UsedPercent,
	}

	return model.WrapResponse(resp), err
}

func (s *Endpoint) DiskHandler(ctx context.Context, input *struct{}) (*struct {
	Body model.Response[[]DiskResponse]
}, error) {
	var resp []DiskResponse
	if parts, err := disk.PartitionsWithContext(ctx, true); err == nil {
		resp = lo.FilterMap(parts, func(p disk.PartitionStat, _ int) (DiskResponse, bool) {
			du, usageErr := disk.UsageWithContext(ctx, p.Mountpoint)
			if usageErr != nil {
				return DiskResponse{}, false
			}
			return DiskResponse{
				Device:     p.Device,
				Mountpoint: p.Mountpoint,
				Total:      du.Total,
				Used:       du.Used,
				Free:       du.Free,
				UsedPct:    du.UsedPercent,
			}, true
		})
	}
	return model.WrapResponse(resp), nil
}

func (s *Endpoint) NetHandler(ctx context.Context, input *struct{}) (*struct {
	Body model.Response[[]NetResponse]
}, error) {
	var resp []NetResponse
	if ni, err := net.IOCountersWithContext(ctx, true); err == nil {
		resp = lo.Map(ni, func(n net.IOCountersStat, _ int) NetResponse {
			return NetResponse{
				Name:        n.Name,
				BytesSent:   n.BytesSent,
				BytesRecv:   n.BytesRecv,
				PacketsSent: n.PacketsSent,
				PacketsRecv: n.PacketsRecv,
			}
		})
	}
	return model.WrapResponse(resp), nil
}

func (s *Endpoint) InfoHandler(ctx context.Context, input *struct{}) (*struct {
	Body model.Response[InfoResponse]
}, error) {
	h, _ := host.InfoWithContext(ctx)
	l, _ := load.AvgWithContext(ctx)
	resp := InfoResponse{
		Hostname:      h.Hostname,
		Uptime:        h.Uptime,
		OS:            h.OS,
		Platform:      h.Platform,
		Load1:         l.Load1,
		Load5:         l.Load5,
		Load15:        l.Load15,
		KernelVersion: h.KernelVersion,
		KernelArch:    h.KernelArch,
	}
	return model.WrapResponse(resp), nil
}

func (s *Endpoint) ClusterHandler(ctx context.Context, input *struct{}) (*struct {
	Body model.Response[ClusterResponse]
}, error) {
	if s.raft == nil {
		return model.WrapResponse(ClusterResponse{Enabled: false}), nil
	}

	status, err := s.raft.Status()
	if err != nil {
		return nil, err
	}

	resp := ClusterResponse{
		Enabled: status.Enabled,
		NodeID:  status.NodeID,
		Bind:    status.Bind,
		Leader:  status.Leader,
		Role:    status.Role,
		Servers: lo.Map(status.Servers, func(item raft.Server, _ int) ClusterServerResponse {
			return ClusterServerResponse{
				ID:       item.ID,
				Address:  item.Address,
				Suffrage: item.Suffrage,
			}
		}),
	}
	return model.WrapResponse(resp), nil
}

func (s *Endpoint) JoinClusterHandler(ctx context.Context, input *struct {
	Body ClusterJoinRequest
}) (*struct {
	Body model.Response[struct {
		Joined bool `json:"joined"`
	}]
}, error) {
	if s.raft == nil {
		return nil, raft.ErrRaftDisabled
	}
	if err := s.raft.AddVoter(input.Body.ID, input.Body.Address); err != nil {
		return nil, err
	}
	return model.WrapResponse(struct {
		Joined bool `json:"joined"`
	}{
		Joined: true,
	}), nil
}

func (s *Endpoint) RemoveClusterHandler(ctx context.Context, input *struct {
	Body ClusterRemoveRequest
}) (*struct {
	Body model.Response[struct {
		Removed bool `json:"removed"`
	}]
}, error) {
	if s.raft == nil {
		return nil, raft.ErrRaftDisabled
	}
	if err := s.raft.RemoveServer(input.Body.ID); err != nil {
		return nil, err
	}
	return model.WrapResponse(struct {
		Removed bool `json:"removed"`
	}{
		Removed: true,
	}), nil
}
