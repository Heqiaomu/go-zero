package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/Heqiaomu/glog"
	dl "github.com/Heqiaomu/goutil/distributelock"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Allocator interface {
	StartAllocate()
	StopAllocate()
	PsWorker()
}

// Work 阻塞或者长期执行的work，ctx.Done通道有消息即停止
type Work func(ctx context.Context, key Key) error

type Key string

var keyCompile = regexp.MustCompile("^[0-9][0-9_]*[0-9]$")

func (k Key) v() bool {
	return keyCompile.MatchString(k.string())
}

func (k Key) string() string {
	return string(k)
}

func (k Key) parse() int {
	trim := strings.ReplaceAll(k.string(), "_", "")
	atoi, _ := strconv.Atoi(trim)
	return atoi
}

type KeyAcquire func() []Key

type EtcdAllocator struct {
	service       string
	name          string // name use to lock key prefix
	self          string // self key in the etcd
	assemblyLines sync.Map
	checkInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	etcdURLs      []string
	client        *clientv3.Client

	kf   KeyAcquire
	work Work
}

type EtcdAllocatorOpt func(*EtcdAllocator)

// WithCheckInterval check interval option, default is 30s
func WithCheckInterval(duration time.Duration) EtcdAllocatorOpt {
	return func(allocator *EtcdAllocator) {
		allocator.checkInterval = duration
	}
}

// WithEtcdURLs etcd urls options, default get from viper('etcd.host')
func WithEtcdURLs(urls []string) EtcdAllocatorOpt {
	return func(allocator *EtcdAllocator) {
		allocator.etcdURLs = urls
	}
}

// WithContext context options
func WithContext(ctx context.Context) EtcdAllocatorOpt {
	return func(allocator *EtcdAllocator) {
		allocator.ctx = ctx
	}
}

var nameCompile = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9\\.\\-_]{1,31}[a-zA-Z0-9]$")

// NewEtcdAllocator new etcd allocator
// service is name registered to etcd, eg: b20.service.core
// name is the EtcdAllocator's name, this have certain format requirements '^[a-zA-Z][a-zA-Z0-9\.\-_]{1,31}[a-zA-Z0-9]$'
// kf KeyAcquire: the keys array to allocate, return Key must in format '^[0-9][0-9_]*[0-9]$'
// work Work: the work content
func NewEtcdAllocator(service, name, self string, kf KeyAcquire, work Work, opts ...EtcdAllocatorOpt) (*EtcdAllocator, error) {
	if !nameCompile.MatchString(name) {
		return nil, errors.Errorf(fmt.Sprintf("Currently new EtcdAllocator failed, because name [%s] not in correct format '^[a-zA-Z][a-zA-Z0-9\\.\\-_]{1,31}[a-zA-Z0-9]$', please check name", name))
	}
	ea := &EtcdAllocator{
		service:       service,
		name:          name,
		self:          self,
		assemblyLines: sync.Map{},
		checkInterval: time.Second * 30,
		kf:            kf,
		etcdURLs:      viper.GetStringSlice("etcd.host"),
		work:          work,
	}
	for _, opt := range opts {
		opt(ea)
	}
	if ea.ctx == nil {
		ea.ctx = context.Background()
	}
	cctx, cancelFunc := context.WithCancel(ea.ctx)
	ea.ctx = cctx
	ea.cancel = cancelFunc
	cli, err := clientv3.NewFromURLs(ea.etcdURLs)
	if err != nil {
		return nil, errors.Wrap(err, "new etcd client")
	}
	ea.client = cli
	return ea, nil
}

// StartAllocate 主要逻辑：从etcd中选择所有的服务，然后选中自己的，查询到index，开始分配，30s检查一次，如果不是自己了就通知停止work
func (ea *EtcdAllocator) StartAllocate() {
	go func() {
		ticker := time.NewTicker(ea.checkInterval)
		for {
			select {
			case <-ticker.C:
				err := ea.checkAllocate()
				if err != nil {
					log.Errorf("Currently check allocate rule failed, Err: [%v]", err)
				}
			case <-ea.ctx.Done():
				log.Info("Currently allocator stop work")
				return
			}
		}
	}()
}

// StopAllocate stop allocate
func (ea *EtcdAllocator) StopAllocate() {
	ea.cancel()
	ea.client.Close()
}

// PsWorker get worker
func (ea *EtcdAllocator) PsWorker() {
	ea.assemblyLines.Range(func(key, value interface{}) bool {
		log.Infof("Currently worker key [%s], work name [%s]", key, value.(*worker).name)
		return true
	})
}

func (ea *EtcdAllocator) checkAllocate() error {
	index, err := ea.getSelfIndex(ea.service)
	if err != nil {
		return errors.Wrap(err, "get self index")
	}
	keys := ea.kf()
	var km = make(map[string]struct{}, 0)
	for _, k := range keys {
		if !k.v() {
			log.Errorf("Currently key [%s] not in correct format '^[0-9][0-9_]*[0-9]$'", k)
			continue
		}
		km[k.string()] = struct{}{}
		self := k.parse()%index.AllCount == index.Index
		load, ok := ea.assemblyLines.Load(k.string())
		if ok && !self {
			// 已经再工作的worker，但是已经不是分配到本服务了，需要停止
			load.(*worker).Stop()
			ea.assemblyLines.Delete(k.string())
		} else if !ok && self {
			// 没有相应的worker，但是新分配到本服务的，需要新建worker
			e := ea.newWorkerAndDo(k)
			if e != nil {
				log.Errorf("Currently new worker [%s] failed, Err: [%v]", k.string(), e)
			}
		}
	}
	// 有些 key 已经删除了，需要停止
	ea.assemblyLines.Range(func(key, value interface{}) bool {
		if _, ok := km[key.(string)]; !ok {
			value.(*worker).Stop()
			ea.assemblyLines.Delete(key.(string))
		}
		return true
	})
	return nil
}

type worker struct {
	id   string
	name string
	ctx  context.Context
	stop context.CancelFunc
	wk   Work
	done chan bool
}

func (w *worker) Stop() {
	w.stop()
	<-w.done
}

func (ea *EtcdAllocator) newWorker(key Key) *worker {
	cctx, cancelFunc := context.WithCancel(ea.ctx)
	return &worker{
		id:   uuid.New().String(),
		name: fmt.Sprintf("%s-worker", key),
		ctx:  cctx,
		stop: cancelFunc,
		wk:   ea.work,
		done: make(chan bool),
	}
}

func (ea *EtcdAllocator) newWorkerAndDo(key Key) error {
	lock, err := dl.NewDistributeLock(fmt.Sprintf("%s_%s", ea.name, key), dl.WithTimeout(time.Second*2), dl.WithEtcdURLs(ea.etcdURLs))
	if err != nil {
		return errors.Wrap(err, "new distribute lock")
	}

	unLockFunc, done, err := lock.Lock()
	if err != nil {
		return errors.Wrap(err, "try lock")
	}
	wkr := ea.newWorker(key)
	go func() {
		select {
		case <-done:
			// 防止 lock 被动中断
			log.Infof("Currently key [%s] lock done", key)
			wkr.Stop()
		case <-wkr.done:
			// 监听工人是否结束工作
			log.Infof("Currently work [%s] done", key)
		}
		ea.assemblyLines.Delete(key.string())
	}()
	// async to work
	go func() {
		defer unLockFunc()
		defer close(wkr.done)
		log.Debugf("Currently work name [%s], id [%s] start", wkr.name, wkr.id)
		e := wkr.wk(wkr.ctx, key)
		if e != nil {
			log.Errorf("Currently do work err, Err: [%v]", e)
			return
		}
		log.Debugf("Currently work name [%s], id [%s] stop", wkr.name, wkr.id)
	}()
	ea.assemblyLines.Store(key.string(), wkr)
	return nil
}

type IndexInfo struct {
	ServiceName string
	Index       int
	AllCount    int
}

func (ea *EtcdAllocator) getSelfIndex(prefix string) (*IndexInfo, error) {
	list, err := listWithKeyPrefix(prefix+"/", ea.client)
	if err != nil {
		return nil, errors.Wrap(err, "list")
	}
	var keys = make([]string, 0)
	for key := range list {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for ind, key := range keys {
		key = key[:strings.LastIndex(key, "/")]
		if key == ea.self {
			return &IndexInfo{
				ServiceName: key,
				Index:       ind,
				AllCount:    len(keys),
			}, nil
		}
	}
	return nil, fmt.Errorf("fail to get self index, because no service(%s) found, please check", prefix)
}

func listWithKeyPrefix(prefix string, cli *clientv3.Client) (map[string]interface{}, error) {
	get, err := cli.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, errors.Wrap(err, "get from etcd with prefix key")
	}
	var kvInfo = make(map[string]interface{}, 0)
	for _, kv := range get.Kvs {
		var m = map[string]interface{}{}
		err := json.Unmarshal(kv.Value, &m)
		if err != nil {
			return nil, err
		}
		kvInfo[string(kv.Key)] = m
	}
	return kvInfo, nil
}
