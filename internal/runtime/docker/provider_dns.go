package docker

import (
	"context"

	"github.com/arcgolabs/collectionx/list"
	"github.com/docker/docker/api/types/container"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type workloadDNSResolver interface {
	WorkloadNameserver() (string, bool)
	WorkloadSearchDomains(namespace string) *list.List[string]
}

// ApplyWorkloadDNS injects orch DNS resolver settings into Docker host config.
func ApplyWorkloadDNS(hostCfg *container.HostConfig, resolver workloadDNSResolver, namespace string) {
	if hostCfg == nil || resolver == nil {
		return
	}
	nameserver, ok := resolver.WorkloadNameserver()
	if !ok {
		return
	}
	hostCfg.DNS = []string{nameserver}
	if search := resolver.WorkloadSearchDomains(namespace); search.Len() > 0 {
		hostCfg.DNSSearch = search.Values()
	}
}

func (p *Provider) recordDockerWorkloadDNS(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload, name string, inspect container.InspectResponse) error {
	ip := primaryIPv4(inspect.NetworkSettings)
	if ip == "" {
		return oopsx.B("runtime", "docker").Errorf("docker: no ipv4 address for container %s (ensure default bridge / or set networkMode)", name)
	}
	if err := p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, ip); err != nil {
		return oopsx.B("runtime", "dns").Wrapf(err, "upsert workload DNS")
	}
	return nil
}

func primaryIPv4(ns *container.NetworkSettings) string {
	if ns == nil {
		return ""
	}
	for _, nw := range ns.Networks {
		if nw != nil && nw.IPAddress != "" {
			return nw.IPAddress
		}
	}
	return ""
}
