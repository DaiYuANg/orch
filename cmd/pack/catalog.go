package main

type packInfo struct {
	Name        string
	Version     string
	Runtime     string
	Description string
}

var builtinCatalog = []packInfo{
	{
		Name:        "postgresql",
		Version:     "17",
		Runtime:     "docker",
		Description: "PostgreSQL database service",
	},
	{
		Name:        "redis",
		Version:     "7",
		Runtime:     "docker",
		Description: "Redis cache service",
	},
	{
		Name:        "minio",
		Version:     "latest",
		Runtime:     "docker",
		Description: "S3-compatible object storage",
	},
	{
		Name:        "nats",
		Version:     "2",
		Runtime:     "docker",
		Description: "NATS messaging server",
	},
}
