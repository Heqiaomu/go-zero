package discovery

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/Heqiaomu/glog"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/zeromicro/go-zero/zrpc/pkg/apisix"
	"github.com/zeromicro/go-zero/zrpc/pkg/etcd"
	"github.com/zeromicro/go-zero/zrpc/pkg/kong"
	"github.com/zeromicro/go-zero/zrpc/pkg/traefik"
)

type ServiceInfo struct {
	SwaggerJson []string
}

// ServicesWatch is the handle of all services watch.it can be create by NewServiceWatch
type ServicesWatch struct {
	Endpoints     []string
	KongAddr      string
	ApisixAddr    []string
	ServicePrefix string
	K             *kong.Kong
	Apisix        *apisix.Apisix
	Traefik       *traefik.Traefik
	WatchEtcd     *etcd.WatchEtcd
	OneWatch      *etcd.OneWatch
	ServicesMtx   sync.Mutex
	ServicesInfo  map[string]*ServiceInfo
}

//get 所有的service，然后进行watch所有service
//根据watch的情况，对kong进行service转发注册

func (sw *ServicesWatch) watchKeyFunc(eventType int32, key string, value string, createRevision int64, modRevision int64) interface{} {
	fmt.Println(eventType, key, createRevision, modRevision)

	serviceKey := strings.Split(key, "/")
	service := serviceKey[0]    // service(for example, "b20.service.pay")
	target := serviceKey[1][5:] // target(for example ,"192.0.2.1:25:8080")

	fmt.Println("service:", service, "target:", target)

	// key put
	if eventType == 0 {
		sw.addServicesInfo(service, value)
		sw.putTraefik(service, target)
	} else {
		// 判断etcd上是否还有其余相同的service/x-target
		var nCount int //相同service/x-target数量
		etcdKV, _ := sw.WatchEtcd.GetWithPrefix(service)
		for etcdKey := range etcdKV {
			n := strings.Index(etcdKey, target)
			if n > 0 {
				nCount++
			}
		}

		// 如果有相同的target,则返回
		if nCount > 0 {
			return nil
		}

		s := strings.Split(service, ".")
		del, _ := sw.delTraefikService(s[2], "http://"+target)
		if del {
			sw.delServicesInfo(service)
		}
	}

	return nil
}

// NewServiceWatch new a ServiceWatch
func NewServiceWatch(endpoints []string, servicePrefix string) (serviceWatch *ServicesWatch, err error) {

	serviceWatch = &ServicesWatch{Endpoints: endpoints, ServicePrefix: servicePrefix, ServicesInfo: make(map[string]*ServiceInfo), Traefik: &traefik.Traefik{}}

	serviceWatch.WatchEtcd, err = etcd.New(endpoints, 3*time.Second)
	serviceWatch.Traefik.WatchEtcd = serviceWatch.WatchEtcd
	if err != nil {
		return nil, err
	}
	return serviceWatch, nil
}

//Close close a ServiceWatch
func (sw *ServicesWatch) Close() {
	sw.WatchEtcd.UnWath(sw.OneWatch)
	sw.WatchEtcd.Close()
}

// Watch watch all services
func (sw *ServicesWatch) Watch() (err error) {
	// 删除所有的routers,services,middleware
	sw.WatchEtcd.DelWithPrefix("traefik")

	// 注册middleware
	sw.Traefik.AddMiddlewareStripPrefix()
	sw.Traefik.AddMiddlewareStripPrefixRegex()
	oauthServerList, err := GetHttpAddrs(viper.GetStringSlice("etcd.host"), viper.GetString("service.prefix")+"oauth")
	if err != nil {
		log.Errorf("Currently get oauth server list failed, err: %v", err)
		return errors.Wrap(err, "Currently get oauth server list failed")
	}
	sw.Traefik.AddMiddlewareForwardauth(oauthServerList)

	//监听service动态情况
	sw.OneWatch, err = sw.WatchEtcd.WatchWithPrefix(sw.ServicePrefix, sw.watchKeyFunc)
	if err != nil {
		fmt.Println("watch service key failed")
		return errors.Wrap(err, "Currently watch service key failed")
	}

	//获取所有的service name
	err = sw.GetAllService()
	if err != nil {
		return errors.Wrap(err, "Currently get all service failed")
	}
	// 添加所有的服务
	sw.AddTraefik()

	return nil
}

func (sw *ServicesWatch) GetAllService() (err error) {

	keyValue, err := sw.WatchEtcd.GetWithPrefix(sw.ServicePrefix)
	if err != nil {
		fmt.Printf("etcd get %s failed\n", sw.ServicePrefix)
		return err
	}

	//key (for example, "b20.service.pay/192.0.2.1:25:8080")
	for key, value := range keyValue {

		serviceKey := strings.Split(key, "/")
		serviceName := serviceKey[0] //serviceName (for example, "b20.service.pay")

		sw.addServicesInfo(serviceName, value)
	}

	return nil
}

// 保存所有的服务名
func (sw *ServicesWatch) addServicesInfo(serviceName string, etcdValue string) {
	sr := &ServiceRegister{}
	json.Unmarshal([]byte(etcdValue), sr)

	sw.ServicesMtx.Lock()
	sw.ServicesInfo[serviceName] = &ServiceInfo{SwaggerJson: sr.SwaggerJson}
	sw.ServicesMtx.Unlock()
}

func (sw *ServicesWatch) delServicesInfo(serviceName string) {

	sw.ServicesMtx.Lock()
	delete(sw.ServicesInfo, serviceName)
	sw.ServicesMtx.Unlock()
}

func (sw *ServicesWatch) addUpstreams() {
	for service := range sw.ServicesInfo {
		upstreamId, err := sw.K.AddUpstream(service)
		if err != nil {
			fmt.Printf("add upstream %s failed\n", service)
			fmt.Println(err)
			continue
		}
		fmt.Println("add upstream:", upstreamId)
	}
}

func (sw *ServicesWatch) addTargets() {
	for service := range sw.ServicesInfo {
		etcdKV, err := sw.WatchEtcd.GetWithPrefix(service)
		if err != nil {
			fmt.Printf("etcd getwithprefix %s failed", service)
			fmt.Println(err)
			continue
		}

		for etcdKey := range etcdKV {
			//serviceKey (for example, "b20.service.pay/192.0.2.1:25:8080")
			serviceKey := strings.Split(etcdKey, "/")
			if len(serviceKey[1]) <= 5 {
				continue
			}
			serviceAddr := serviceKey[1][5:] //serviceName (for example, "192.0.2.1:25:8080")

			targets, err := sw.K.ListTarget(service)
			if err != nil {
				fmt.Printf("listTarget %s failed\n", service)
				fmt.Println(err)
				continue
			}

			//判断target是否已经注册
			alreadyRegister := false
			for _, target := range targets.Data {
				if target.Target == serviceAddr {
					alreadyRegister = true
					break
				}
			}

			//target没有注册即进行注册
			if !alreadyRegister {
				targetId, err := sw.K.AddTarget(service, serviceAddr, 100)
				if err != nil {
					fmt.Printf("add upstreamId/%s/target %s failed", service, serviceAddr)
					fmt.Println(err)
					continue
				}
				fmt.Println("add targetId:", targetId)
			}
		}
	}
}

func (sw *ServicesWatch) addServices() {
	for service := range sw.ServicesInfo {
		serviceId, err := sw.K.AddService(service, "http://"+service)
		if err != nil {
			fmt.Printf("add service %s failed\n", service)
			fmt.Println(err)
			continue
		}
		fmt.Println("add serviceId:", serviceId)
	}
}

func (sw *ServicesWatch) addRoutes() {
	for service := range sw.ServicesInfo {
		routesId, err := sw.K.AddRoutes(service, kong.Methods, kong.Protocols, []string{"/" + service})
		if err != nil {
			fmt.Printf("add service/%s/routes failed\n", service)
			fmt.Println(err)
			continue
		}
		fmt.Println("add routesId:", routesId)
	}
}

// addKong 注册kong
func (sw *ServicesWatch) addKong() {
	//注册upstream
	sw.addUpstreams()

	//注册targets
	sw.addTargets()

	//注册service
	sw.addServices()

	//注册routes
	sw.addRoutes()
}

// 服务启动时删除所有的路由，后续通过AddApisix和watch重新添加。
// 目前采用gateway重启来保证routes完整性，准确性。
func (sw *ServicesWatch) DelApisix() error {

	err := sw.Apisix.DelAllRoutes()
	if err != nil {
		fmt.Println(err.Error())
	}

	err = sw.Apisix.DelAllUpstreams()
	if err != nil {
		fmt.Println(err.Error())
	}

	return err
}

// addKong 注册kong
func (sw *ServicesWatch) AddApisix() error {
	for service := range sw.ServicesInfo {
		etcdKV, err := sw.WatchEtcd.GetWithPrefix(service)
		if err != nil {
			fmt.Printf("etcd getwithprefix %s failed", service)
			fmt.Println(err)
			continue
		}

		// 增加upstreams
		for etcdKey := range etcdKV {
			//serviceKey (for example, "b20.service.pay/192.0.2.1:25:8080")
			serviceKey := strings.Split(etcdKey, "/")
			if len(serviceKey[1]) <= 5 {
				continue
			}
			serviceAddr := serviceKey[1][5:] //serviceName (for example, "192.0.2.1:25:8080")

			if err = sw.putProxy(service, serviceAddr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (sw *ServicesWatch) putTarget(service string, target string) {
	sw.K.AddUpstream(service)

	// add target
	targets, err := sw.K.ListTarget(service)
	if err != nil {
		fmt.Printf("listTarget %s failed\n", service)
		fmt.Println(err)
		return
	}
	//判断target是否已经注册
	alreadyRegister := false
	for _, v := range targets.Data {
		if v.Target == target {
			alreadyRegister = true
			break
		}
	}
	if alreadyRegister == false {
		sw.K.AddTarget(service, target, 100)
	}

	sw.K.AddService(service, "http://"+service)
	sw.K.AddRoutes(service, kong.Methods, kong.Protocols, []string{"/" + service})
}

func (sw *ServicesWatch) delTarget(service string, target string) (remainder int, err error) {
	sw.K.DelTarge(service, target)

	targets, err := sw.K.ListTarget(service)

	remainder = len(targets.Data)

	if err != nil {
		return remainder, err
	}

	if len(targets.Data) == 0 {
		sw.K.DelUpstream(service)
		sw.K.DelService(service)
	}
	return remainder, nil
}

func (sw *ServicesWatch) putProxy(service string, target string) error {
	upstreamID, err := sw.Apisix.AddUpstreams(service, target, 10)
	if err != nil {
		return err
	}

	routesID, err := sw.Apisix.GetRoutes(service)
	if err != nil {
		return err
	}
	if routesID == "" {
		_, err := sw.Apisix.AddRoutes(service, upstreamID)
		if err != nil {
			return err
		}
	}
	return nil
}

// traefik services
func (sw *ServicesWatch) addTraefikService(service string, target string) error {
	targets, err := sw.Traefik.WatchEtcd.GetWithPrefix("traefik/http/services/blocface-" + service + "/loadBalancer/servers")
	if err != nil {
		return err
	}
	if targets == nil {
		return sw.Traefik.WatchEtcd.Put("traefik/http/services/blocface-"+service+"/loadBalancer/servers/0/url", "http://"+target)
	} else {
		for _, v := range targets {
			if v == target {
				return nil
			}
		}
		d := len(targets)
		return sw.Traefik.WatchEtcd.Put(fmt.Sprintf("traefik/http/services/blocface-"+service+"/loadBalancer/servers/%d/url", d), "http://"+target)
	}
}

// traefik services
// del-表示是否整个服务都要被删除，代表该服务没有了
func (sw *ServicesWatch) delTraefikService(service string, target string) (del bool, err error) {
	targets, err := sw.Traefik.WatchEtcd.GetWithPrefix("traefik/http/services/blocface-" + service + "/loadBalancer/servers")
	if err != nil {
		return false, err
	}

	sortTargets := make([]string, 0)
	for _, v := range targets {
		if v != target {
			sortTargets = append(sortTargets, v)
		}
	}
	sort.Strings(sortTargets)

	sw.Traefik.WatchEtcd.DelWithPrefix(fmt.Sprintf("traefik/http/services/blocface-" + service + "/loadBalancer/servers/"))

	for i, url := range sortTargets {
		sw.Traefik.WatchEtcd.Put(fmt.Sprintf("traefik/http/services/blocface-"+service+"/loadBalancer/servers/%d/url", i), url)
	}
	if len(sortTargets) == 0 {
		return true, sw.delTraefikRouter(service)
	}

	return false, nil
}

func (sw *ServicesWatch) addTraefikRouter(service string) error {
	err := sw.Traefik.WatchEtcd.Put("traefik/http/routers/blocface-"+service+"/rule", fmt.Sprintf("PathPrefix(`/%s%s`)", viper.GetString("service.prefix"), service))
	if err != nil {
		return errors.Wrap(err, "put traefik router")
	}
	if !strings.Contains(service, "ws-") &&
		!strings.Contains(service, "socketio-") {
		sw.Traefik.WatchEtcd.Put("traefik/http/routers/blocface-"+service+"/middlewares/0", "middleware-forwardauth")
		sw.Traefik.WatchEtcd.Put("traefik/http/routers/blocface-"+service+"/middlewares/1", "middleware-stripprefixregex")
	} else {
		sw.Traefik.WatchEtcd.Put("traefik/http/routers/blocface-"+service+"/middlewares/0", "middleware-stripprefixregex")
	}
	return sw.Traefik.WatchEtcd.Put("traefik/http/routers/blocface-"+service+"/service", "blocface-"+service)
}

func (sw *ServicesWatch) delTraefikRouter(service string) error {
	return sw.Traefik.WatchEtcd.DelWithPrefix("traefik/http/routers/blocface-" + service)
}

func (sw *ServicesWatch) putTraefik(service string, target string) error {
	s := strings.Split(service, ".")
	sw.addTraefikRouter(s[2])
	return sw.addTraefikService(s[2], target)
}

// AddTraefik
func (sw *ServicesWatch) AddTraefik() error {
	for service := range sw.ServicesInfo {
		etcdKV, err := sw.WatchEtcd.GetWithPrefix(service)
		if err != nil {
			fmt.Printf("etcd getwithprefix %s failed", service)
			fmt.Println(err)
			continue
		}

		for etcdKey := range etcdKV {
			//serviceKey (for example, "b20.service.pay/yt67-192.0.2.1:25:8080")
			serviceKey := strings.Split(etcdKey, "/")
			if len(serviceKey[1]) <= 5 {
				continue
			}
			serviceAddr := serviceKey[1][5:] //serviceName (for example, "192.0.2.1:25:8080")

			sw.putTraefik(service, serviceAddr)
			log.Infof("%s增加路由:%s", service, serviceAddr)
		}
	}
	return nil
}
