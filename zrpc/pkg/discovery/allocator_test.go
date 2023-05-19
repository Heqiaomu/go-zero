package discovery

import (
	"context"
	log "github.com/Heqiaomu/glog"
	dl "github.com/Heqiaomu/goutil/distributelock"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"reflect"
	"testing"
	"time"
)

func TestNewEtcdAllocator(t *testing.T) {
	mock := func(t *testing.T) *gomonkey.Patches {
		patches := gomonkey.NewPatches()
		patches.ApplyFunc(clientv3.NewFromURLs, func(urls []string) (*clientv3.Client, error) {
			return &clientv3.Client{}, nil
		})
		return patches
	}
	log.Logger()
	t.Run("test NewEtcdAllocator", func(t *testing.T) {
		mk := mock(t)
		defer mk.Reset()
		allocator, err := NewEtcdAllocator("b20.service.chainctl", "block-collect", "self", func() []Key {
			return []Key{}
		}, func(ctx context.Context, key Key) error {
			return nil
		}, WithContext(context.Background()), WithEtcdURLs([]string{}), WithCheckInterval(time.Second))
		assert.Nil(t, err)
		assert.NotNil(t, allocator)
	})
}

type mockKV2 struct {
}

func (m mockKV2) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	panic("implement me")
}

var count int

func (m mockKV2) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	count++
	if count%2 == 0 {
		return &clientv3.GetResponse{
			Header: nil,
			Kvs: []*mvccpb.KeyValue{
				{
					Key: []byte("self/pp"),
				},
				{
					Key: []byte("other/pp"),
				},
			},
			More:  false,
			Count: 0,
		}, nil
	}
	return &clientv3.GetResponse{
		Header: nil,
		Kvs: []*mvccpb.KeyValue{
			{
				Key: []byte("self/pp"),
			},
		},
		More:  false,
		Count: 0,
	}, nil
}

func (m mockKV2) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	panic("implement me")
}

func (m mockKV2) Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	panic("implement me")
}

func (m mockKV2) Do(ctx context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	panic("implement me")
}

func (m mockKV2) Txn(ctx context.Context) clientv3.Txn {
	panic("implement me")
}

func TestStartAllocate(t *testing.T) {
	mock := func(t *testing.T) *gomonkey.Patches {
		patches := gomonkey.NewPatches()
		//var ea *EtcdAllocator
		//patches.ApplyMethod(reflect.TypeOf(ea),"getSelfIndex", func(ea *EtcdAllocator, prefix string) (*IndexInfo, error) {
		//	count++
		//	if count%2 == 0 {
		//		return &IndexInfo{
		//			ServiceName: "b20.service.chainctl",
		//			Index:       0,
		//			AllCount:    2,
		//		}, nil
		//	}
		//	return &IndexInfo{
		//		ServiceName: "b20.service.chainctl",
		//		Index:       0,
		//		AllCount:    1,
		//	}, nil
		//})
		patches.ApplyFunc(clientv3.NewFromURLs, func(urls []string) (*clientv3.Client, error) {
			return &clientv3.Client{KV: mockKV2{}}, nil
		})
		var c *clientv3.Client
		patches.ApplyMethod(reflect.TypeOf(c), "Close", func(c *clientv3.Client) error {
			return nil
		})

		patches.ApplyFunc(dl.NewDistributeLock, func(key string, opts ...dl.DistributeLockOpt) (*dl.DistributeLock, error) {
			return &dl.DistributeLock{}, nil
		})
		var dll *dl.DistributeLock
		patches.ApplyMethod(reflect.TypeOf(dll), "Lock", func(dll *dl.DistributeLock) (unLockFunc dl.UnLock, done <-chan struct{}, err error) {
			return func() error {
				t.Logf("unlock")
				return nil
			}, make(chan struct{}), nil
		})
		return patches
	}
	log.Logger()
	t.Run("test StartAllocate", func(t *testing.T) {
		mk := mock(t)
		defer mk.Reset()
		timeout, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
		defer cancelFunc()
		var cc int
		allocator, err := NewEtcdAllocator("b20.service.chainctl", "block-collect", "self", func() []Key {
			cc++
			if cc%2 == 0 {
				return []Key{"10_1"}
			}
			return []Key{"10_1", "10_2"}
		}, func(ctx context.Context, key Key) error {
			return nil
		}, WithContext(timeout), WithEtcdURLs([]string{}), WithCheckInterval(time.Second))
		assert.Nil(t, err)
		assert.NotNil(t, allocator)
		allocator.StartAllocate()
		time.Sleep(time.Second * 6)
		allocator.PsWorker()
		allocator.StopAllocate()
	})
}

func TestCom(t *testing.T) {
	matchString := keyCompile.MatchString("1_0")
	t.Logf("%t", matchString)
}

//func TestNewEtcdAllocator(t *testing.T) {
//	mock := func(t *testing.T) *gomonkey.Patches {
//		patches := gomonkey.NewPatches()
//		patches.ApplyFunc(getSelfIndex, func(prefix string) (*IndexInfo, error) {
//			assert.Equal(t, prefix, "b20.service.chainctl")
//			second := time.Now().Second()
//			t.Logf("这里的时间%d", second)
//			time.Sleep(time.Second)
//			if second%2 == 0 {
//				t.Logf("这里返回了2个")
//				return &IndexInfo{
//					ServiceName: "b20.service.chainctl",
//					Index:       0,
//					AllCount:    2,
//				}, nil
//			}
//			return &IndexInfo{
//				ServiceName: "b20.service.chainctl",
//				Index:       0,
//				AllCount:    1,
//			}, nil
//		})
//		return patches
//	}
//	log.Logger()
//	var count int
//	t.Run("test NewEtcdAllocator", func(t *testing.T) {
//		mk := mock(t)
//		defer mk.Reset()
//		cancel, cancelFunc := context.WithCancel(context.Background())
//		defer cancelFunc()
//		allocator, err := NewEtcdAllocator("b20.service.chainctl", "block-collect", func() []Key {
//			if count > 0 {
//				time.Sleep(10*time.Second)
//				return []Key{
//					"10_1",
//					"10_2",
//				}
//			}
//			count ++
//			return []Key{
//				"10_1",
//				"10_2",
//				"10_3",
//				"10_4",
//				"10_5",
//			}
//		}, func(ctx context.Context, key string) error {
//			ticker := time.NewTicker(time.Second * 10)
//			for {
//				select {
//				case <-ticker.C:
//					s := uuid.New().String()
//					t.Logf("开始工作了， key : [%s], uuid [%s]", key, s)
//					time.Sleep(time.Second*20)
//					t.Logf("结束工作了， key : [%s], uuid [%s]", key, s)
//				case <-ctx.Done():
//					return nil
//				}
//			}
//		}, WithContext(cancel), WithEtcdURLs([]string{"127.0.0.1:2379"}), WithCheckInterval(time.Second*3))
//		assert.Nil(t, err)
//		allocator.StartAllocate()
//		time.Sleep(time.Minute*2)
//		allocator.StopAllocate()
//		time.Sleep(time.Second)
//	})
//}
//
