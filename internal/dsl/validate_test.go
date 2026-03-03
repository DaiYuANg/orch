package dsl

import "testing"

func TestValidateWorkload(t *testing.T) {
	w := &Workload{
		Name: "demo",
		Units: []Unit{
			{
				Name: "backend",
				Tasks: []Task{
					{
						Name:    "api",
						Type:    "service",
						Driver:  "docker",
						Image:   "nginx:latest",
						Network: &NetworkConfig{Port: map[string]int{"http": 8080}},
						Check: &HealthCheck{
							Type:     "http",
							Path:     "/health",
							Interval: "5s",
							Timeout:  "2s",
							Retries:  3,
						},
					},
				},
			},
		},
	}

	if err := ValidateWorkload(w); err != nil {
		t.Fatalf("validate workload: %v", err)
	}
}

func TestValidateWorkloadInvalid(t *testing.T) {
	w := &Workload{Name: "demo"}
	if err := ValidateWorkload(w); err == nil {
		t.Fatalf("expected validation error")
	}
}
