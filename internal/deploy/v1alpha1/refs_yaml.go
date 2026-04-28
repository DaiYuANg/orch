package v1alpha1

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/daiyuang/orch/pkg/oopsx"
)

func (r *WorkloadRef) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		r.Name = strings.TrimSpace(value.Value)
		return nil
	case yaml.MappingNode:
		type alias WorkloadRef
		var a alias
		if err := value.Decode(&a); err != nil {
			return oopsx.B("deploy").Wrapf(err, "decode workload ref")
		}
		*r = WorkloadRef(a)
		return nil
	case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
		return oopsx.B("deploy").Errorf("invalid workload ref (unsupported YAML node kind %v)", value.Kind)
	default:
		return oopsx.B("deploy").Errorf("invalid workload ref")
	}
}

func (r *VolumeRef) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		r.Name = strings.TrimSpace(value.Value)
		return nil
	case yaml.MappingNode:
		type alias VolumeRef
		var a alias
		if err := value.Decode(&a); err != nil {
			return oopsx.B("deploy").Wrapf(err, "decode volume ref")
		}
		*r = VolumeRef(a)
		return nil
	case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
		return oopsx.B("deploy").Errorf("invalid volume ref (unsupported YAML node kind %v)", value.Kind)
	default:
		return oopsx.B("deploy").Errorf("invalid volume ref")
	}
}

func (r *EndpointRef) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		s := strings.TrimSpace(value.Value)
		if s == "" {
			return nil
		}
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 {
			return oopsx.B("deploy").Errorf("invalid endpoint ref %q (expected workload:endpoint)", s)
		}
		r.Workload = strings.TrimSpace(parts[0])
		r.Endpoint = strings.TrimSpace(parts[1])
		return nil
	case yaml.MappingNode:
		type alias EndpointRef
		var a alias
		if err := value.Decode(&a); err != nil {
			return oopsx.B("deploy").Wrapf(err, "decode endpoint ref")
		}
		*r = EndpointRef(a)
		return nil
	case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
		return oopsx.B("deploy").Errorf("invalid endpoint ref (unsupported YAML node kind %v)", value.Kind)
	default:
		return oopsx.B("deploy").Errorf("invalid endpoint ref")
	}
}
