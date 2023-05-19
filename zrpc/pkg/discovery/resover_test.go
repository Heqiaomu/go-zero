package discovery

import (
	"context"
	log "github.com/Heqiaomu/glog"
	gomonkey "github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"reflect"
	"testing"
)

// import "testing"
//
//	func TestGetHttpAddrs(t *testing.T) {
//		etcdAddrs := []string{"172.22.66.86:2379"}
//		addrs,err := GetHttpAddrs(etcdAddrs,"b20.service.nats")
//		if err != nil {
//			t.Error(err)
//		}
//		t.Log(addrs)
//	}
type mockKV struct {
}

func (m mockKV) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	panic("implement me")
}

func (m mockKV) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{
		Header: nil,
		Kvs: []*mvccpb.KeyValue{
			{
				Value: []byte("{\"Addr\":\"abc\"}"),
			},
		},
		More:  false,
		Count: 0,
	}, nil
}

func (m mockKV) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	panic("implement me")
}

func (m mockKV) Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	panic("implement me")
}

func (m mockKV) Do(ctx context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	return clientv3.OpResponse{}, nil
}

func (m mockKV) Txn(ctx context.Context) clientv3.Txn {
	panic("implement me")
}

func TestGetGrpcAddr(t *testing.T) {
	mock := func(t *testing.T) *gomonkey.Patches {
		patches := gomonkey.NewPatches()
		patches.ApplyFunc(clientv3.NewFromURLs, func(urls []string) (*clientv3.Client, error) {
			return &clientv3.Client{KV: mockKV{}}, nil
		})
		var c *clientv3.Client
		patches.ApplyMethod(reflect.TypeOf(c), "Close", func(_ *clientv3.Client) error {
			return nil
		})
		return patches
	}
	log.Logger()
	t.Run("test GetGrpcAddr", func(t *testing.T) {
		mk := mock(t)
		defer mk.Reset()
		addr, err := GetGrpcAddr([]string{}, "")
		assert.Nil(t, err)
		assert.NotNil(t, addr)
	})
}
