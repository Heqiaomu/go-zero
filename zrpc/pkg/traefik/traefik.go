package traefik

import (
	"github.com/zeromicro/go-zero/zrpc/pkg/etcd"
)

type Traefik struct {
	WatchEtcd *etcd.WatchEtcd
}

func (t *Traefik) AddMiddlewareStripPrefix() error {
	return t.WatchEtcd.Put("traefik/http/middlewares/middleware-stripprefix/stripPrefix/prefixes", "/b20.service.gateway")
}

func (t *Traefik) AddMiddlewareStripPrefixRegex() error {
	return t.WatchEtcd.Put("traefik/http/middlewares/middleware-stripprefixregex/stripprefixregex/regex", "/b20.service.*?/")
}

func (t *Traefik) AddMiddlewareForwardauth(endpoints []string) error {
	//auths := viper.GetStringSlice("traefik.forwardauth")
	t.WatchEtcd.Put("traefik/http/middlewares/middleware-forwardauth/forwardAuth/address", "http://"+endpoints[0]+"/api/v1.0/auth")
	t.WatchEtcd.Put("traefik/http/middlewares/middleware-forwardauth/forwardAuth/trustForwardHeader", "true")
	return t.WatchEtcd.Put("traefik/http/middlewares/middleware-forwardauth/forwardAuth/authResponseHeaders", "X-Forwarded-Auth-User")
}
