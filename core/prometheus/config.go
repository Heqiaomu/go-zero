package prometheus

// A Config is a prometheus config.
type Config struct {
	Enable bool   `json:"enable,default=false"`
	Host   string `json:"host,optional"`
	Port   int    `json:"port,default=9101"`
	Path   string `json:"path,default=/metrics"`
}
