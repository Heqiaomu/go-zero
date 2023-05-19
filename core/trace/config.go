package trace

// TraceName represents the tracing name.
const TraceName = "go-zero"

// A Config is a opentelemetry config.
type Config struct {
	Name     string  `json:"name,optional"`
	Endpoint string  `json:"endpoint,optional"`
	Sampler  float64 `json:"sampler,default=1.0"`
	Batcher  string  `json:"batcher,default=jaeger,options=jaeger|zipkin"`
}
