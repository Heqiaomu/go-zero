package discovery

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"
	"sync"
	"time"

	log "github.com/Heqiaomu/glog"
	bletcd "github.com/zeromicro/go-zero/zrpc/pkg/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdSvcer struct {
	endpoints []string
	svcName   string
	lock      *sync.Mutex
	httpAddrs []string
	watcher   *bletcd.WatchEtcd
}

// NewEtcdSvcAddrProvider get a etcd service addr client which provide http addrs
// @endpoints etcd endpoints, e.g. []string{"127.0.0.1"}
// @svcName service name which has been registered in etcd as a key prefix, e.g. "b20.service.mysql"
func NewEtcdSvcer(endpoints []string, svcName string) (*EtcdSvcer, error) {
	svcer := &EtcdSvcer{endpoints: endpoints, svcName: svcName, httpAddrs: make([]string, 0), lock: &sync.Mutex{}}

	// init addr
	err := svcer.initSvcer()
	if err != nil {
		return nil, err
	}

	// watch svc
	err = svcer.watch()
	if err != nil {
		return nil, err
	}

	return svcer, nil
}

// GetHttpAddrs get all http addrs
func (e *EtcdSvcer) GetHttpAddrs() []string {
	return e.httpAddrs
}

// GetHttpAddr get a random http addr
func (e *EtcdSvcer) GetHttpAddr() string {
	e.lock.Lock()
	defer e.lock.Unlock()

	if len(e.httpAddrs) == 0 {
		return ""
	}
	rand.Seed(time.Now().Unix())
	index := rand.Intn(len(e.httpAddrs))
	return e.httpAddrs[index]
}

// Close close the watcher and rm the svcer
func (e *EtcdSvcer) Close() {
	e.watcher.Close()
}

func (e *EtcdSvcer) initSvcer() error {
	cli, err := clientv3.NewFromURLs(e.endpoints)
	if err != nil {
		return err
	}
	defer cli.Close()

	get, err := cli.Get(context.Background(), e.svcName, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	kvs := get.Kvs
	if len(kvs) == 0 {
		return nil
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	for _, kv := range kvs {
		httpAddr := getHttpAddrFromEtcdValue(string(kv.Value))
		e.httpAddrs = append(e.httpAddrs, httpAddr)
	}
	return nil
}

func (e *EtcdSvcer) watch() error {
	// new watcher
	watcher, err := bletcd.New(e.endpoints, time.Duration(10)*time.Second)
	if err != nil {
		return err
	}
	e.watcher = watcher
	// watch svc
	_, err = watcher.WatchWithPrefix(e.svcName, e.etcdSvcAddrWatcher)
	if err != nil {
		return err
	}
	return nil
}

func (svcer *EtcdSvcer) etcdSvcAddrWatcher(eventType int32, key string, value string, createRevision int64, modRevision int64) interface{} {
	ss := strings.Split(key, "/")
	kv, err := svcer.watcher.GetWithPrefix(ss[0])
	if err != nil {
		return nil
	}
	tempAddrs := make([]string, 0)
	for _, v := range kv {
		tempAddrs = append(tempAddrs, getHttpAddrFromEtcdValue(v))
	}
	svcer.httpAddrs = tempAddrs
	log.Infof("Updating server[%s] addr", ss[0])
	return nil
}

func getSvcNameFormEtcdKey(key string) string {
	ss := strings.Split(key, "/")
	if len(ss) == 2 {
		return ss[0]
	}
	return ""
}

func getHttpAddrFromEtcdKey(key string) string {
	ss := strings.Split(key, "/")
	if len(ss) == 2 && len(ss[1]) >= 5 {
		return ss[1][5:]
	}
	return ""
}

// Deprecated
func getHttpAddrFromEtcdValue(value string) string {
	s := ServiceRegister{}
	err := json.Unmarshal([]byte(value), &s)
	if err != nil {
		log.Errorf("can not unmarshal from etcd value %v", err)
		return ""
	}
	return s.HttpAddr
}
