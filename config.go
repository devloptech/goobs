package goobs

type Config struct {
	ServiceName     string
	Environment     string
	OtelEndpoint    string
	EnableMetrics   bool
	SkipCallerPkgs  []string
	SkipCallerFiles []string
}
