package discov

import (
	"errors"
	"strings"
)

// EtcdConf is the config item with the given key on etcd.
type EtcdConf struct {
	Hosts              []string               `json:"host"`
	Key                string                 `json:"key"`
	User               string                 `json:"user,optional"`
	Pass               string                 `json:"pass,optional"`
	CertFile           string                 `json:"certFile,optional"`
	CertKeyFile        string                 `json:"certKeyFile,optional=CertFile"`
	CACertFile         string                 `json:"caCertFile,optional=CertFile"`
	InsecureSkipVerify bool                   `json:"insecureSkipVerify,optional"`
	Extra              map[string]interface{} `json:"extra,optional"`
}

// HasAccount returns if account provided.
func (c EtcdConf) HasAccount() bool {
	return len(c.User) > 0 && len(c.Pass) > 0
}

// HasTLS returns if TLS CertFile/CertKeyFile/CACertFile are provided.
func (c EtcdConf) HasTLS() bool {
	return len(c.CertFile) > 0 && len(c.CertKeyFile) > 0 && len(c.CACertFile) > 0
}

// Validate validates c.
func (c *EtcdConf) Validate() error {
	if len(c.Hosts) == 0 {
		return errors.New("empty etcd hosts")
	} else if len(c.Key) == 0 {
		return errors.New("empty etcd key")
	} else {
		// 去掉协议头，兼容
		var newHost []string
		for _, host := range c.Hosts {
			host = strings.Trim(host, "http://")
			host = strings.Trim(host, "https://")
			newHost = append(newHost, host)
		}
		c.Hosts = newHost
		return nil
	}
}

// GetExtra get extra
func (c EtcdConf) GetExtra(key string) interface{} {
	return c.Extra[key]
}
