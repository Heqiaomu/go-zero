package discov

import (
	"context"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/discov/internal"
	"github.com/zeromicro/go-zero/core/lang"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/syncx"
	"github.com/zeromicro/go-zero/core/threading"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type (
	// PubOption defines the method to customize a Publisher.
	PubOption func(client *Publisher)

	// A Publisher can be used to publish the value to an etcd cluster on the given key.
	Publisher struct {
		endpoints  []string
		key        string
		customKey  bool
		fullKey    string
		id         int64
		value      string
		lease      clientv3.LeaseID
		quit       *syncx.DoneChan
		pauseChan  chan lang.PlaceholderType
		resumeChan chan lang.PlaceholderType

		// b20 暂时没用到
		StopSignal bool
	}

	// SrvDiscovery is the operating handle of discovery
	SrvDiscovery struct {
		Timeout      time.Duration
		Client       *clientv3.Client
		Mtx          sync.Mutex
		Name         string
		SrvRegisters map[string]*SrvRegister
	}

	// SrvRegister is the info of one service register
	SrvRegister struct {
		ServiceName string
		HttpAddr    string
		InstanceId  string // etcd注册时的实例ID
		StopSignal  bool
		LeaseId     clientv3.LeaseID
		CancelFunc  context.CancelFunc
	}
)

// NewPublisher returns a Publisher.
// endpoints is the hosts of the etcd cluster.
// key:value are a pair to be published.
// opts are used to customize the Publisher.
func NewPublisher(endpoints []string, key, value string, opts ...PubOption) *Publisher {
	publisher := &Publisher{
		endpoints:  endpoints,
		key:        key,
		value:      value,
		quit:       syncx.NewDoneChan(),
		pauseChan:  make(chan lang.PlaceholderType),
		resumeChan: make(chan lang.PlaceholderType),
	}

	for _, opt := range opts {
		opt(publisher)
	}

	return publisher
}

// B20KeepAlive keeps key:value alive.
func (p *Publisher) B20KeepAlive() error {
	serviceKey := p.fullKey

	go func() {
		for !p.StopSignal {
			cli, err := internal.GetRegistry().GetConn(p.endpoints)
			if err != nil {
				logx.Error("get connect error", p.lease)
				continue
			}
			defer cli.Close()

			if 0 != p.lease {
				cli.Revoke(cli.Ctx(), p.lease)
				logx.Error("revoke error", p.lease)
				p.lease = 0
			}

			p.lease, err = p.register(cli)
			if err != nil {
				logx.Error("register error", p.lease)
				continue
			}

			leaseKeepAliveResponse, err := cli.KeepAlive(cli.Ctx(), p.lease)
			if err != nil {
				logx.Error("etcd KeepAlive lease failed")
				continue
			}
			logx.Info("register a serviceKey:", serviceKey)
			logx.Info("lease ", p.lease)
		rspOk:
			for {
				select {
				case _, ok := <-leaseKeepAliveResponse:
					if !ok {
						if p.StopSignal {
							cli.Revoke(context.Background(), p.lease)
							logx.Info("revoke ", p.lease)
							logx.Info("unregister ", serviceKey)
							return
						}
						logx.Info("etcd is disconnected")
						break rspOk
					}
				}
			}
		}

		logx.Info("go func register is stop")
	}()

	return nil
}

// KeepAlive keeps key:value alive.
func (p *Publisher) KeepAlive() error {
	cli, err := internal.GetRegistry().GetConn(p.endpoints)
	if err != nil {
		return err
	}

	p.lease, err = p.register(cli)
	if err != nil {
		return err
	}

	proc.AddWrapUpListener(func() {
		p.Stop()
	})

	if p.customKey {
		p.Watch(cli)
	}

	return p.keepAliveAsync(cli)
}

// Pause pauses the renewing of key:value.
func (p *Publisher) Pause() {
	p.pauseChan <- lang.Placeholder
}

// Resume resumes the renewing of key:value.
func (p *Publisher) Resume() {
	p.resumeChan <- lang.Placeholder
}

// Stop stops the renewing and revokes the registration.
func (p *Publisher) Stop() {
	p.quit.Close()
}

func (p *Publisher) Watch(cli internal.EtcdClient) {
	watch := cli.Watch(cli.Ctx(), p.fullKey)
	threading.GoSafe(func() {
		for {
			select {
			case wresp, ok := <-watch:
				if !ok {
					logx.Error("etcd monitor chan has been closed")
					return
				}
				if wresp.Canceled {
					logx.Errorf("etcd monitor chan has been canceled, error: %v", wresp.Err())
					return
				}
				if wresp.Err() != nil {
					logx.Error("etcd monitor chan error: %v", wresp.Err())
					return
				}
				for _, ev := range wresp.Events {
					switch ev.Type {
					case clientv3.EventTypeDelete:
						p.revoke(cli)
						return
					default:
						continue
					}
				}
			case <-p.quit.Done():
				return
			}
		}
	})
	return
}
func (p *Publisher) keepAliveAsync(cli internal.EtcdClient) error {
	ch, err := cli.KeepAlive(cli.Ctx(), p.lease)
	if err != nil {
		return err
	}

	threading.GoSafe(func() {
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					p.revoke(cli)
					if err := p.KeepAlive(); err != nil {
						logx.Errorf("KeepAlive: %s", err.Error())
					}
					return
				}
			case <-p.pauseChan:
				logx.Infof("paused etcd renew, key: %s, value: %s", p.key, p.value)
				p.revoke(cli)
				select {
				case <-p.resumeChan:
					if err := p.KeepAlive(); err != nil {
						logx.Errorf("KeepAlive: %s", err.Error())
					}
					return
				case <-p.quit.Done():
					return
				}
			case <-p.quit.Done():
				p.revoke(cli)
				return
			}
		}
	})

	return nil
}

func (p *Publisher) register(client internal.EtcdClient) (clientv3.LeaseID, error) {
	resp, err := client.Grant(client.Ctx(), TimeToLive)
	if err != nil {
		return clientv3.NoLease, err
	}

	lease := resp.ID
	if p.id > 0 {
		p.fullKey = makeEtcdKey(p.key, p.id)
	} else {
		p.fullKey = makeEtcdKey(p.key, int64(lease))
	}
	_, err = client.Put(client.Ctx(), p.fullKey, p.value, clientv3.WithLease(lease))

	return lease, err
}

func (p *Publisher) revoke(cli internal.EtcdClient) {
	if _, err := cli.Revoke(cli.Ctx(), p.lease); err != nil {
		logx.Error(err)
	}
}

// WithId customizes a Publisher with the id.
func WithId(id int64) PubOption {
	return func(publisher *Publisher) {
		publisher.id = id
	}
}

// WithPubEtcdAccount provides the etcd username/password.
func WithPubEtcdAccount(user, pass string) PubOption {
	return func(pub *Publisher) {
		RegisterAccount(pub.endpoints, user, pass)
	}
}

// WithPubEtcdTLS provides the etcd CertFile/CertKeyFile/CACertFile.
func WithPubEtcdTLS(certFile, certKeyFile, caFile string, insecureSkipVerify bool) PubOption {
	return func(pub *Publisher) {
		logx.Must(RegisterTLS(pub.endpoints, certFile, certKeyFile, caFile, insecureSkipVerify))
	}
}

// WithCustomKV customizes a Publisher with the key and value.
func WithCustomKV(etcd EtcdConf) PubOption {
	// @param publisher key: origin service name, e.g. chainctl
	// @param publisher value: origin service http address, e.g. 127.0.0.1:8080
	// @return publisher key: blocface format, e.g. b20.service.chainctl/1dd2-127.0.0.1:8080
	// @return publisher value: blocface format, struct ServiceRegister json format
	return func(publisher *Publisher) {
		publisher.key = etcd.GetExtra("key").(string)
		publisher.value = etcd.GetExtra("value").(string)
		publisher.customKey = true
	}
}
