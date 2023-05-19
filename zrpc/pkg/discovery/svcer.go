package discovery

type Svcer interface {
	GetHttpAddrs() []string
	GetHttpAddr() string
	Close()
}
