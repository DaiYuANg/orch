package dsl

import (
	"fmt"
	"strings"
	"time"
)

func ValidateWorkload(w *Workload) error {
	if w == nil {
		return fmt.Errorf("workload is nil")
	}
	if strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("workload.name is required")
	}
	if len(w.Units) == 0 {
		return fmt.Errorf("workload.units must not be empty")
	}

	for unitIdx, unit := range w.Units {
		if strings.TrimSpace(unit.Name) == "" {
			return fmt.Errorf("workload.units[%d].name is required", unitIdx)
		}
		if len(unit.Tasks) == 0 {
			return fmt.Errorf("workload.units[%d].tasks must not be empty", unitIdx)
		}

		for taskIdx, task := range unit.Tasks {
			loc := fmt.Sprintf("workload.units[%d].tasks[%d]", unitIdx, taskIdx)
			if err := validateTask(loc, task); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateTask(loc string, task Task) error {
	if strings.TrimSpace(task.Name) == "" {
		return fmt.Errorf("%s.name is required", loc)
	}
	if strings.TrimSpace(task.Type) == "" {
		return fmt.Errorf("%s.type is required", loc)
	}
	driver := strings.ToLower(strings.TrimSpace(task.Driver))
	if driver == "" {
		return fmt.Errorf("%s.driver is required", loc)
	}
	if driver != "docker" {
		return fmt.Errorf("%s.driver=%q is unsupported now, only docker is enabled", loc, task.Driver)
	}
	if strings.TrimSpace(task.Image) == "" {
		return fmt.Errorf("%s.image is required when driver=docker", loc)
	}
	if task.Replicas < 0 {
		return fmt.Errorf("%s.replicas must be >= 0", loc)
	}
	if task.Check == nil {
		return nil
	}
	return validateHealthCheck(loc+".check", task.Check)
}

func validateHealthCheck(loc string, check *HealthCheck) error {
	checkType := strings.ToLower(strings.TrimSpace(check.Type))
	if checkType == "" {
		return fmt.Errorf("%s.type is required", loc)
	}
	switch checkType {
	case "http":
		if strings.TrimSpace(check.Path) == "" {
			return fmt.Errorf("%s.path is required when type=http", loc)
		}
	case "cmd":
		if strings.TrimSpace(check.Command) == "" {
			return fmt.Errorf("%s.command is required when type=cmd", loc)
		}
	default:
		return fmt.Errorf("%s.type=%q is unsupported, allowed: http|cmd", loc, check.Type)
	}

	if check.Interval != "" {
		if _, err := time.ParseDuration(check.Interval); err != nil {
			return fmt.Errorf("%s.interval parse error: %w", loc, err)
		}
	}
	if check.Timeout != "" {
		if _, err := time.ParseDuration(check.Timeout); err != nil {
			return fmt.Errorf("%s.timeout parse error: %w", loc, err)
		}
	}
	if check.Retries < 0 {
		return fmt.Errorf("%s.retries must be >= 0", loc)
	}
	return nil
}
