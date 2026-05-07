package v1alpha1

import (
	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"
)

func (a *App) WorkloadList() *list.List[Workload] {
	if a == nil {
		return list.NewList[Workload]()
	}
	return list.NewList(a.Workloads...)
}

func (a *App) ConfigList() *list.List[Config] {
	if a == nil {
		return list.NewList[Config]()
	}
	return list.NewList(a.Configs...)
}

func (a *App) SecretList() *list.List[Secret] {
	if a == nil {
		return list.NewList[Secret]()
	}
	return list.NewList(a.Secrets...)
}

func (a *App) VolumeList() *list.List[Volume] {
	if a == nil {
		return list.NewList[Volume]()
	}
	return list.NewList(a.Volumes...)
}

func (a *App) IngressList() *list.List[Ingress] {
	if a == nil {
		return list.NewList[Ingress]()
	}
	return list.NewList(a.Ingresses...)
}

func (m Metadata) LabelMap() *mapping.Map[string, string] {
	return mapping.NewMapFrom(m.Labels)
}

func (m Metadata) AnnotationMap() *mapping.Map[string, string] {
	return mapping.NewMapFrom(m.Annotations)
}

func (w Workload) DependsOnList() *list.List[WorkloadRef] {
	return list.NewList(w.DependsOn...)
}

func (w Workload) EndpointList() *list.List[Endpoint] {
	return list.NewList(w.Endpoints...)
}

func (w Workload) MountList() *list.List[Mount] {
	return list.NewList(w.Mounts...)
}

func (w Workload) EnvList() *list.List[EnvVar] {
	return list.NewList(w.Run.Env...)
}

func (w Workload) PreferredNodeList() *list.List[string] {
	if w.Scheduling == nil {
		return list.NewList[string]()
	}
	return list.NewList(w.Scheduling.PreferredNodes...)
}

func (ing Ingress) RouteList() *list.List[IngressRoute] {
	return list.NewList(ing.Routes...)
}

func (c Config) DataMap() *mapping.Map[string, string] {
	return mapping.NewMapFrom(c.Data)
}

func (s Secret) DataMap() *mapping.Map[string, string] {
	return mapping.NewMapFrom(s.Data)
}

func (o *DockerOptions) LabelMap() *mapping.Map[string, string] {
	if o == nil {
		return mapping.NewMap[string, string]()
	}
	return mapping.NewMapFrom(o.Labels)
}
