package system

import (
	"github.com/DaiYuANg/warden/internal/http/model"
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
		lo.ForEach(parts, func(p disk.PartitionStat, index int) {
			if du, err := disk.UsageWithContext(ctx, p.Mountpoint); err == nil {
				resp = append(resp, DiskResponse{
					Device:     p.Device,
					Mountpoint: p.Mountpoint,
					Total:      du.Total,
					Used:       du.Used,
					Free:       du.Free,
					UsedPct:    du.UsedPercent,
				})
			}
		})
	}
	return model.WrapResponse(resp), nil
}

func (s *Endpoint) NetHandler(ctx context.Context, input *struct{}) (*struct {
	Body model.Response[[]NetResponse]
}, error) {
	var resp []NetResponse
	if ni, err := net.IOCountersWithContext(ctx, true); err == nil {
		for _, n := range ni {
			resp = append(resp, NetResponse{
				Name:        n.Name,
				BytesSent:   n.BytesSent,
				BytesRecv:   n.BytesRecv,
				PacketsSent: n.PacketsSent,
				PacketsRecv: n.PacketsRecv,
			})
		}
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
