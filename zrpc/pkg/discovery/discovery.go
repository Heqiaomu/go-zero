package discovery

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	log "github.com/Heqiaomu/glog"
	"github.com/pkg/errors"
)

var (
	sdSlice []*SrvDiscovery
	fixUUID = "abcd"
)

// RegisterService4zrpc register service for zrpc on etcd
func RegisterService4zrpc(etcdEndpoint []string, serviceKey, serviceValue string) (err error) {
	sd, err := NewDiscovery(etcdEndpoint, 3*time.Second)
	if err != nil {
		return err
	}

	err = sd.RegisterWithKV(serviceKey, serviceValue)
	if err != nil {
		return err
	}
	sdSlice = append(sdSlice, sd)
	log.Infof("Currently registered %d services", len(sdSlice))
	return
}

// RegisterService register service on etcd
func RegisterService(moduleName string, etcdEndpoint []string, servicePrefix string, sr ServiceRegister) (err error) {
	sd, err := NewDiscovery(etcdEndpoint, 3*time.Second)
	if err != nil {
		return err
	}

	var swagger []string
	//s, err := ioutil.ReadFile(moduleName + ".swagger.json")
	//if err != nil {
	//	fmt.Println(moduleName + ".swagger.json is not exist")
	//} else {
	//	swagger = append(swagger, string(s))
	//}
	//
	//h, err := ioutil.ReadFile(moduleName + ".http.swagger.json")
	//if err != nil {
	//	fmt.Println(moduleName + ".http.swagger.json is not exist")
	//} else {
	//	swagger = append(swagger, string(h))
	//}

	err = sd.Register(servicePrefix+moduleName, sr.Addr, sr.Metadata, sr.HttpAddr, swagger)
	if err != nil {
		return err
	}
	sdSlice = append(sdSlice, sd)
	log.Infof("Currently registered %d services", len(sdSlice))
	return
}

// RegisterServiceFixedUUID register service on etcd
func RegisterServiceFixedUUID(moduleName string, etcdEndpoint []string, servicePrefix string, sr ServiceRegister) (err error) {
	sd, err := NewDiscovery(etcdEndpoint, 3*time.Second)
	if err != nil {
		return errors.Wrap(err, "get discovery")
	}

	var swagger []string
	s, err := ioutil.ReadFile(moduleName + ".swagger.json")
	if err != nil {
		fmt.Println(moduleName + ".swagger.json is not exist")
	} else {
		swagger = append(swagger, string(s))
	}

	h, err := ioutil.ReadFile(moduleName + ".http.swagger.json")
	if err != nil {
		fmt.Println(moduleName + ".http.swagger.json is not exist")
	} else {
		swagger = append(swagger, string(h))
	}

	err = sd.Register(servicePrefix+moduleName, sr.Addr, sr.Metadata, sr.HttpAddr, swagger, fixUUID)
	if err != nil {
		return errors.Wrap(err, "register fixed uuid")
	}
	sdSlice = append(sdSlice, sd)
	log.Infof("Currently registered %d services", len(sdSlice))
	return
}

// UnRegisterService unRegister service on etcd
func UnRegisterService() {

	for _, v := range sdSlice {
		if v != nil {
			v.Close()
		}
	}
	return
}

// UnRegisterOneService unRegister one service on etcd
func UnRegisterOneService(serviceName string) {
	var sdSliceTmp = make([]*SrvDiscovery, 0)
	for _, v := range sdSlice {
		if strings.Contains(v.Name, serviceName) {
			v.Close()
		} else {
			sdSliceTmp = append(sdSliceTmp, v)
		}
	}
	sdSlice = sdSliceTmp
}

// ListService list blocface service
func ListService(etcdEndpoint []string, prefix string) (map[string]interface{}, error) {
	sd, err := NewDiscovery(etcdEndpoint, 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer sd.Close()
	return sd.List(prefix)
}
