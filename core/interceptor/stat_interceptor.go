package interceptor

// A StatInterceptorConf is a state interceptor config.
type StatInterceptorConf struct {
	RequestFilter         map[string]bool `json:"request_filter,optional"`
	RequestContentFilter  map[string]bool `json:"request_content_filter,optional"`
	ResponseContentFilter map[string]bool `json:"response_content_filter,optional"`
}
