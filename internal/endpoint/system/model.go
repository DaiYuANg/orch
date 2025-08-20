package system

type CPUResponse struct {
	ModelName string    `json:"model_name"`
	Cores     int32     `json:"cores"`
	Percent   []float64 `json:"percent"`
}

type MemResponse struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

type DiskResponse struct {
	Device     string  `json:"device"`
	Mountpoint string  `json:"mountpoint"`
	Total      uint64  `json:"total"`
	Used       uint64  `json:"used"`
	Free       uint64  `json:"free"`
	UsedPct    float64 `json:"used_percent"`
}

type NetResponse struct {
	Name        string `json:"name"`
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
}

type InfoResponse struct {
	Hostname      string  `json:"hostname"`
	Uptime        uint64  `json:"uptime"`
	OS            string  `json:"os"`
	Load1         float64 `json:"load1"`
	Load5         float64 `json:"load5"`
	Load15        float64 `json:"load15"`
	Platform      string  `json:"platform"`
	KernelVersion string  `json:"kernelVersion"`
	KernelArch    string  `json:"kernelArch"`
}
