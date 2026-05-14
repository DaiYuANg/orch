package composeimport

import (
	"fmt"
	"strings"

	composetypes "github.com/compose-spec/compose-go/v2/types"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func mountsFromCompose(service string, vols []composetypes.ServiceVolumeConfig, rep *Report) ([]deployv1.Mount, []deployv1.Volume) {
	var mounts []deployv1.Mount
	var extraVol []deployv1.Volume

	for i, v := range vols {
		mount, extra, ok := mountFromCompose(service, i, v, rep)
		if !ok {
			continue
		}
		mounts = append(mounts, mount)
		if extra != nil {
			extraVol = append(extraVol, *extra)
		}
	}
	return mounts, extraVol
}

func mountFromCompose(service string, idx int, v composetypes.ServiceVolumeConfig, rep *Report) (deployv1.Mount, *deployv1.Volume, bool) {
	target := strings.TrimSpace(v.Target)
	if target == "" {
		return deployv1.Mount{}, nil, false
	}
	switch strings.ToLower(strings.TrimSpace(v.Type)) {
	case "", "volume":
		return volumeMountFromCompose(service, idx, target, v, rep)
	case "bind":
		return bindMountFromCompose(service, idx, target, v, rep), bindVolumeFromCompose(service, idx), true
	case "tmpfs":
		rep.warnf("service %q: tmpfs mount %q skipped", service, target)
		return deployv1.Mount{}, nil, false
	default:
		rep.warnf("service %q: volume type %q not mapped", service, strings.TrimSpace(v.Type))
		return deployv1.Mount{}, nil, false
	}
}

func volumeMountFromCompose(
	service string,
	idx int,
	target string,
	v composetypes.ServiceVolumeConfig,
	rep *Report,
) (deployv1.Mount, *deployv1.Volume, bool) {
	source := strings.TrimSpace(v.Source)
	if source == "" {
		rep.warnf("service %q: volumes[%d] anonymous volume not mapped", service, idx)
		return deployv1.Mount{}, nil, false
	}
	return deployv1.Mount{
		Volume:   deployv1.VolumeRef{Name: source},
		Target:   target,
		ReadOnly: v.ReadOnly,
	}, nil, true
}

func bindMountFromCompose(service string, idx int, target string, v composetypes.ServiceVolumeConfig, rep *Report) deployv1.Mount {
	volume := bindVolumeFromCompose(service, idx)
	rep.warnf("service %q: bind mount %q -> %q mapped as named volume %q (host path not in canonical volume model yet)",
		service, strings.TrimSpace(v.Source), target, volume.Name)
	return deployv1.Mount{
		Volume:   deployv1.VolumeRef{Name: volume.Name},
		Target:   target,
		ReadOnly: v.ReadOnly,
	}
}

func bindVolumeFromCompose(service string, idx int) *deployv1.Volume {
	return &deployv1.Volume{Name: fmt.Sprintf("bind-%s-%d", service, idx)}
}
