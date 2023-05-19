package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/google/uuid"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// SrvRegister is the info of one service register
type SrvRegister struct {
	ServiceName string
	HttpAddr    string
	InstanceId  string // etcd注册时的实例ID
	StopSignal  bool
	LeaseId     clientv3.LeaseID
	CancelFunc  context.CancelFunc
}

// SrvDiscovery is the operating handle of discovery
type SrvDiscovery struct {
	Timeout      time.Duration
	Client       *clientv3.Client
	Mtx          sync.Mutex
	Name         string
	SrvRegisters map[string]*SrvRegister
}

// NewDiscovery new a SrvDiscovery .endpoints : etcd cluster addr
func NewDiscovery(endpoints []string, timeout time.Duration) (sd *SrvDiscovery, err error) {
	client, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: timeout})
	if err != nil {
		logx.Errorf("create etcd3 client failed: %v", err)
		return nil, err
	}

	sd = &SrvDiscovery{
		Timeout:      timeout,
		Client:       client,
		SrvRegisters: make(map[string]*SrvRegister)}

	return sd, nil
}

// Close close SrvDiscovery
func (sd *SrvDiscovery) Close() {

	for _, srvRegister := range sd.SrvRegisters {
		sd.UnRegister(srvRegister.ServiceName, srvRegister.HttpAddr, srvRegister.InstanceId)
	}

	for {
		if len(sd.SrvRegisters) > 0 {
			time.Sleep(1 * time.Millisecond)
			continue
		}
		sd.Client.Close()
		break
	}
}

// ServiceRegister is the format of etcd
type ServiceRegister struct {
	Addr        string   `json:"Addr"`
	Metadata    string   `json:"Metadata"`
	HttpAddr    string   `json:"HttpAddr"`
	SwaggerJson []string `json:"SwaggerJson"`
}

func genServiceKey(serviceName, httpAddr string, params ...string) string {
	var instanceId, serviceKey string
	if params != nil && len(params) > 0 {
		instanceId = "abcd"
		serviceKey = fmt.Sprintf("%s/%s-%s", serviceName, instanceId, "255.255.255.255")
	} else {
		instanceId = uuid.New().String()[0:4]
		serviceKey = fmt.Sprintf("%s/%s-%s", serviceName, instanceId, httpAddr)
	}

	return serviceKey
}

func parseServiceKey(serviceKey string) (serviceName, httpAddr, instanceId string) {
	res := strings.Split(serviceKey, "/")
	if len(res) >= 2 {
		serviceName = res[0]
		uuidHttpAddr := strings.Split(res[1], "-")
		if len(uuidHttpAddr) >= 2 {
			instanceId = uuidHttpAddr[0]
			httpAddr = strings.Join(uuidHttpAddr[1:], "-")
		}
	}
	return
}

// RegisterWithKV register service on etcd .string form of address (for example, "192.0.2.1:25:1726")
func (sd *SrvDiscovery) RegisterWithKV(serviceKey, serviceValue string) error {
	sd.Name = serviceKey
	serviceName, httpAddr, instanceId := parseServiceKey(serviceKey)
	{
		sd.Mtx.Lock()
		sd.SrvRegisters[serviceKey] = &SrvRegister{StopSignal: false, ServiceName: serviceName, HttpAddr: httpAddr, InstanceId: instanceId}
		sd.Mtx.Unlock()
	}

	go func() {
		for !sd.SrvRegisters[serviceKey].StopSignal {

			var etcdCtx context.Context
			if 0 != sd.SrvRegisters[serviceKey].LeaseId {
				if sd.SrvRegisters[serviceKey].CancelFunc != nil {
					sd.SrvRegisters[serviceKey].CancelFunc()
				}
				etcdCtx, _ = context.WithTimeout(context.Background(), sd.Timeout)
				sd.Client.Revoke(etcdCtx, sd.SrvRegisters[serviceKey].LeaseId)
				logx.Infof("revoke %s", sd.SrvRegisters[serviceKey].LeaseId)

				sd.SrvRegisters[serviceKey].LeaseId = 0
				sd.SrvRegisters[serviceKey].CancelFunc = nil
			}

			etcdCtx, _ = context.WithTimeout(context.Background(), sd.Timeout)
			leaseResp, err := sd.Client.Grant(etcdCtx, 10)
			if err != nil {
				logx.Errorf("etcd grant failed, err:[%v]", err)
				continue
			}
			sd.SrvRegisters[serviceKey].LeaseId = leaseResp.ID

			etcdCtx, _ = context.WithTimeout(context.Background(), sd.Timeout)
			_, err = sd.Client.Put(etcdCtx, serviceKey, serviceValue, clientv3.WithLease(leaseResp.ID))
			if err != nil {
				logx.Errorf("etcd put serviceKey failed, err:[%v]", err)
				continue
			}

			etcdCtx, cancelFunc := context.WithCancel(context.Background())
			sd.SrvRegisters[serviceKey].CancelFunc = cancelFunc

			leaseKeepAliveResponse, err := sd.Client.KeepAlive(etcdCtx, leaseResp.ID)
			if err != nil {
				logx.Errorf("etcd KeepAlive lease failed, err:[%v]", err)
				continue
			}
			logx.Infof("register a serviceKey:[%s]", serviceKey)
			logx.Infof("lease [%v]", leaseResp.ID)
		rspOk:
			for {
				select {
				case _, ok := <-leaseKeepAliveResponse:
					if !ok {

						if sd.SrvRegisters[serviceKey].StopSignal {
							sd.Client.Revoke(context.Background(), sd.SrvRegisters[serviceKey].LeaseId)
							logx.Infof("revoke [%v]", sd.SrvRegisters[serviceKey].LeaseId)
							logx.Infof("unregister [%v]", serviceKey)

							{
								sd.Mtx.Lock()
								delete(sd.SrvRegisters, serviceKey)
								sd.Mtx.Unlock()
							}
							return
						}
						logx.Error("etcd is disconnected")
						break rspOk
					}
				}
			}
		}
		// 没有连接上etcd时，退出时需要删除map的key
		{
			sd.Mtx.Lock()
			delete(sd.SrvRegisters, serviceKey)
			sd.Mtx.Unlock()
		}
		logx.Error("go func register is stop")
	}()

	// wait goroutinue finished
	return nil
}

// Register register service on etcd .string form of address (for example, "192.0.2.1:25:1726")
func (sd *SrvDiscovery) Register(serviceName string, address string, metadata string, httpAddr string, swaggerJson []string, params ...string) (err error) {
	serviceKey := genServiceKey(serviceName, httpAddr, params...)
	sr := ServiceRegister{Addr: address, Metadata: metadata, SwaggerJson: swaggerJson, HttpAddr: httpAddr}
	b, err := json.Marshal(sr)
	if err != nil {
		return err
	}
	serviceValue := string(b)

	return sd.RegisterWithKV(serviceKey, serviceValue)
}

// UnRegister unregister service form etcd
func (sd *SrvDiscovery) UnRegister(serviceName string, httpAddr string, instanceId string) {
	serviceKey := fmt.Sprintf("%s/%s-%s", serviceName, instanceId, httpAddr)
	if instanceId == "abcd" {
		serviceKey = fmt.Sprintf("%s/%s-%s", serviceName, instanceId, "255.255.255.255")
	}
	if sd.SrvRegisters[serviceKey] == nil {
		return
	}

	sd.SrvRegisters[serviceKey].StopSignal = true

	if sd.SrvRegisters[serviceKey].CancelFunc != nil {
		sd.SrvRegisters[serviceKey].CancelFunc()
	}

}

// List list blocface services
func (sd *SrvDiscovery) List(prefix string) (map[string]interface{}, error) {
	var ctx = context.Background()
	get, err := sd.Client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
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
