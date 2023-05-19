package config

import (
	"context"
	"fmt"
	"time"

	goetcd "go.etcd.io/etcd/client/v3"
)

const (
	defaultContextTimeout = 30
)

// A RemoteConfigManager retrieves and decrypts configuration from a key/value store.
type RemoteConfigManager interface {
	Get(key string) ([]byte, error)
	Put(key string, value []byte) error
	Watch(key string, stop chan bool) <-chan *Response
}

// standardConfigManager is a RemoteConfigManager instance
type standardConfigManager struct {
	store Store
}

// NewStandardConfigManager new standard config manager
func NewStandardConfigManager(client Store) (RemoteConfigManager, error) {
	return standardConfigManager{client}, nil
}

// NewStandardEtcdConfigManager new standard etcd config manager
func NewStandardEtcdConfigManager(machines []string) (RemoteConfigManager, error) {
	store, err := New(machines)
	if err != nil {
		return nil, err
	}
	return NewStandardConfigManager(store)
}

// Get retrieves and decodes a secconf value stored at key.
func (c standardConfigManager) Get(key string) ([]byte, error) {
	value, err := c.store.Get(key)
	if err != nil {
		return nil, err
	}
	return value, err
}

// Put will put a key/value into the data store
func (c standardConfigManager) Put(key string, value []byte) error {
	err := c.store.Put(key, value)
	return err
}

// Response define store response
type Response struct {
	Value []byte
	Error error
}

// Watch will monitor key in the data sotre
func (c standardConfigManager) Watch(key string, stop chan bool) <-chan *Response {
	resp := make(chan *Response, 0)
	backendResp := c.store.Watch(key, stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case r := <-backendResp:
				if r.Error != nil {
					resp <- &Response{nil, r.Error}
					continue
				}
				resp <- &Response{r.Value, nil}
			}
		}
	}()
	return resp
}

// Store is a K/V store backend that retrieves and sets, and monitors
// data in a K/V store.
type Store interface {
	// Get retrieves a value from a K/V store for the provided key.
	Get(key string) ([]byte, error)

	// Set sets the provided key to value.
	Put(key string, value []byte) error

	// Watch monitors a K/V store for changes to key.
	Watch(key string, stop chan bool) <-chan *Response
}

// Client define a Store instance
type Client struct {
	client    *goetcd.Client
	kv        goetcd.KV
	waitIndex uint64
}

// New return Client implement Store
func New(machines []string) (*Client, error) {
	newClient, err := goetcd.New(goetcd.Config{
		Endpoints: machines,
	})
	if err != nil {
		return nil, fmt.Errorf("creating new etcd client for crypt.backend.Client: %v", err)
	}
	keysAPI := goetcd.NewKV(newClient)
	return &Client{client: newClient, kv: keysAPI, waitIndex: 0}, nil
}

// Get implement etcd v3 get
func (c *Client) Get(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(defaultContextTimeout)*time.Second)
	defer cancel()

	getResponse, err := c.kv.Get(ctx, key, goetcd.WithPrevKV())
	if err != nil {
		return []byte(""), err
	}
	if len(getResponse.Kvs) == 1 {
		return getResponse.Kvs[0].Value, nil
	}

	return []byte(""), nil
}

// Put implement etcd v3 put
func (c *Client) Put(key string, value []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(defaultContextTimeout)*time.Second)
	defer cancel()

	_, err := c.kv.Put(ctx, key, string(value), goetcd.WithPrevKV())
	if err != nil {
		return err
	}

	return nil
}

// Watch implement etcd v3 watch
func (c *Client) Watch(key string, stop chan bool) <-chan *Response {
	// TODO:

	return nil
}
