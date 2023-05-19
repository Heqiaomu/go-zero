package etcd

import (
	"context"
	"fmt"
	log "github.com/Heqiaomu/glog"
	"github.com/pkg/errors"
	"sync"
	"time"

	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// OneWatch is etcd's watch on one key
type OneWatch struct {
	Watcher    clientv3.Watcher
	CancelFunc context.CancelFunc
	WithPrefix bool
	WatchId    uuid.UUID
}

// WatchEtcd is the handle of the etcd,it can watch keys
type WatchEtcd struct {
	Client  *clientv3.Client
	Timeout time.Duration
	Mtx     sync.Mutex
	Watchs  map[uuid.UUID]*OneWatch
}

// New can create a WatchEtcd
func New(endpoints []string, timeout time.Duration) (we *WatchEtcd, err error) {
	client, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: timeout})
	if err != nil {
		fmt.Errorf("create etcd3 client failed: %v", err)
		return nil, err
	}
	we = &WatchEtcd{Client: client, Watchs: make(map[uuid.UUID]*OneWatch), Timeout: timeout}

	i := 0
	for i = 0; i < 10; i++ {
		err = we.Put("blocface/test", "1")
		if err != nil {
			continue
		} else {
			break
		}
	}
	if i == 10 {
		log.Fatal("etcd无法访问，gateway退出")
	}
	return we, nil
}

// Close close WatchEtcd,stop all watch
func (we *WatchEtcd) Close() {

	we.Mtx.Lock()
	for _, value := range we.Watchs {
		we.UnWath(value)
	}
	we.Mtx.Unlock()

	we.Client.Close()
}

// WatchKeyFunc is a callback of etcd's watch
type WatchKeyFunc func(eventType int32, key string, value string, createRevision int64, modRevision int64) interface{}

// Watch key with Prefix .For example, watch 'foo'  can return 'foo1', 'foo2', and so on.
func (we *WatchEtcd) WatchWithPrefix(key string, watchKeyFunc WatchKeyFunc) (oneWatch *OneWatch, err error) {
	return we.watch(key, watchKeyFunc, clientv3.WithPrefix())
}

// Watch key with from key.For example, watch 'foo'  can return '1foo', 'foo2', and so on.
func (we *WatchEtcd) WatchWithFromKey(key string, watchKeyFunc WatchKeyFunc) (oneWatch *OneWatch, err error) {
	return we.watch(key, watchKeyFunc, clientv3.WithFromKey())
}

// Watch key,just the key
func (we *WatchEtcd) WatchKey(key string, watchKeyFunc WatchKeyFunc) (oneWatch *OneWatch, err error) {
	return we.watch(key, watchKeyFunc)
}

// watch 具有断线重连功能，连接断开重连后，不需要二次watch
// watch 可以watch相同的key，返回的oneWatch不是同一个
func (we *WatchEtcd) watch(key string, watchKeyFunc WatchKeyFunc, opts ...clientv3.OpOption) (oneWatch *OneWatch, err error) {
	w := clientv3.NewWatcher(we.Client)

	ctx, cancelFunc := context.WithCancel(context.TODO())
	wid := uuid.New()
	oneWatch = &OneWatch{Watcher: w, CancelFunc: cancelFunc, WatchId: wid, WithPrefix: false}

	{
		we.Mtx.Lock()
		we.Watchs[wid] = oneWatch
		we.Mtx.Unlock()
	}

	watchChan := w.Watch(ctx, key, opts...)

	go func() {
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				watchKeyFunc(int32(event.Type), string(event.Kv.Key), string(event.Kv.Value), event.Kv.CreateRevision, event.Kv.ModRevision)
			}
		}
		fmt.Println("watchChan is stop")
	}()

	return oneWatch, nil
}

// UnWath the OneWatch
func (we *WatchEtcd) UnWath(oneWatch *OneWatch) {

	if oneWatch == nil {
		return
	}
	we.Watchs[oneWatch.WatchId].CancelFunc()
	we.Watchs[oneWatch.WatchId].Watcher.Close()
	{
		we.Mtx.Lock()
		delete(we.Watchs, oneWatch.WatchId)
		we.Mtx.Unlock()
	}
}

// Put put the key and value into the etcd
func (we *WatchEtcd) Put(key string, value string) (err error) {
	kv := clientv3.NewKV(we.Client)

	ctx, _ := context.WithTimeout(context.Background(), we.Timeout)
	_, err = kv.Put(ctx, key, value, clientv3.WithPrevKV())
	if err != nil {
		return err
	}

	return nil
}

// Del del the key from etcd
func (we *WatchEtcd) Del(key string) (err error) {

	kv := clientv3.NewKV(we.Client)

	ctx, _ := context.WithTimeout(context.Background(), we.Timeout)
	_, err = kv.Delete(ctx, key, clientv3.WithPrevKV())
	if err != nil {
		return err
	}

	return nil
}

// Del del the key from etcd
func (we *WatchEtcd) DelWithPrefix(key string) (err error) {

	kv := clientv3.NewKV(we.Client)

	ctx, _ := context.WithTimeout(context.Background(), we.Timeout)
	_, err = kv.Delete(ctx, key, clientv3.WithPrevKV(), clientv3.WithPrefix())
	if err != nil {
		return err
	}

	return nil
}

// Get the key only
func (we *WatchEtcd) Get(key string) (value string, err error) {

	kv := clientv3.NewKV(we.Client)

	ctx, _ := context.WithTimeout(context.Background(), we.Timeout)
	getResponse, err := kv.Get(ctx, key, clientv3.WithPrevKV())
	if err != nil {
		return "", err
	}
	if len(getResponse.Kvs) == 1 {
		return string(getResponse.Kvs[0].Value), nil
	}

	return "", nil
}

// Get keys that are greater than or equal to the given key using byte compare
func (we *WatchEtcd) GetWithFromKey(key string) (keyValue map[string]string, err error) {

	kv := clientv3.NewKV(we.Client)

	ctx, _ := context.WithTimeout(context.Background(), we.Timeout)
	getResponse, err := kv.Get(ctx, key, clientv3.WithPrevKV(), clientv3.WithFromKey())
	if err != nil {
		return nil, err
	}

	keyValue = make(map[string]string)
	for _, rsp := range getResponse.Kvs {
		keyValue[string(rsp.Key)] = string(rsp.Value)
	}

	return keyValue, nil
}

// Get keys with matching prefix
func (we *WatchEtcd) GetWithPrefix(key string) (keyValue map[string]string, err error) {

	kv := clientv3.NewKV(we.Client)

	ctx, _ := context.WithTimeout(context.Background(), we.Timeout)
	getResponse, err := kv.Get(ctx, key, clientv3.WithPrevKV(), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	keyValue = make(map[string]string)
	for _, rsp := range getResponse.Kvs {
		keyValue[string(rsp.Key)] = string(rsp.Value)
	}

	return keyValue, nil
}

// Get keys only with matching prefix
func (we *WatchEtcd) GetWithPrefixAndKeyOnly(key string) (keyValue map[string]string, err error) {

	kv := clientv3.NewKV(we.Client)

	ctx, _ := context.WithTimeout(context.Background(), we.Timeout)
	getResponse, err := kv.Get(ctx, key, clientv3.WithKeysOnly(), clientv3.WithPrefix())
	if err != nil {
		return nil, errors.Wrap(err, "get etcd kv")
	}

	keyValue = make(map[string]string)
	for _, rsp := range getResponse.Kvs {
		keyValue[string(rsp.Key)] = string(rsp.Value)
	}

	return keyValue, nil
}
