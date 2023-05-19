package kong

import (
	"encoding/json"
	"fmt"
	simplejson "github.com/bitly/go-simplejson"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

// Methods describe http method
var Methods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}

// Protocols describe http protocol
var Protocols = []string{"http", "https"}

// ErrRsp is error response of kong
type ErrRsp struct {
	Message string `json:"message"`
	Name    string `json:"name"`
	Code    int    `json:"code"`
}

// Kong is kong operating handle
type Kong struct {
	Addr string
}

// Create return a Kong
func Create(addr string) (k *Kong) {
	k = &Kong{Addr: addr}
	return k
}

// AddUpstream add upstream into kong
func (k *Kong) AddUpstream(service string) (upstreamId string, err error) {

	resp, err := http.PostForm(k.Addr+"/upstreams/",
		url.Values{"name": {service}})

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	return parseHttpRspWithAdd(resp)
}

// DelUpstream del upstream from kong
func (k *Kong) DelUpstream(service string) (err error) {

	req, _ := http.NewRequest("DELETE", k.Addr+"/upstreams/"+service, nil)

	_, err = http.DefaultClient.Do(req)

	if err != nil {
		fmt.Println(err)
	}

	return err
}

// Target is the respone of get target from kong
type Target struct {
	Next string `json:"next"`
	Data []struct {
		CreatedAt float64 `json:"created_at"`
		Upstream  struct {
			ID string `json:"id"`
		} `json:"upstream"`
		ID     string `json:"id"`
		Target string `json:"target"`
		Weight int    `json:"weight"`
	} `json:"data"`
}

// AddTarget add target into kong
func (k *Kong) AddTarget(upstream string, target string, weight int) (targetId string, err error) {

	resp, err := http.PostForm(k.Addr+"/upstreams/"+upstream+"/targets/",
		url.Values{"target": {target}, "weight": {strconv.Itoa(weight)}})

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	return parseHttpRspWithAdd(resp)
}

// DelTarge del target form kong
func (k *Kong) DelTarge(upstream string, target string) (err error) {
	targets, err := k.ListTarget(upstream)
	if err != nil {
		fmt.Println(err)
		return err
	}

	var targetId string
	for _, t := range targets.Data {
		if t.Target == target {
			targetId = t.ID
			break
		}
	}

	if targetId != "" {
		req, _ := http.NewRequest("DELETE", k.Addr+"/upstreams/"+upstream+"/targets/"+targetId, nil)

		http.DefaultClient.Do(req)

		fmt.Println("delete target:", target)
	}

	return nil
}

// ListTarget list all targets of a upstream
func (k *Kong) ListTarget(upstream string) (target *Target, err error) {
	resp, err := http.Get(k.Addr + "/upstreams/" + upstream + "/targets/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	target = &Target{}
	json.Unmarshal(body, target)

	return target, nil
}

// AddService add service into kong
func (k *Kong) AddService(name string, serviceUrl string) (serviceId string, err error) {

	resp, err := http.PostForm(k.Addr+"/services/",
		url.Values{"name": {name}, "url": {serviceUrl}})

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	return parseHttpRspWithAdd(resp)
}

// DelService del service from kong.it need to del routes of service first,then del services
func (k *Kong) DelService(service string) (err error) {

	routesReq, _ := http.NewRequest("DELETE", k.Addr+"/services/"+service+"/routes/"+"routes-"+service, nil)

	_, err = http.DefaultClient.Do(routesReq)

	if err != nil {
		fmt.Println(err)
	}

	servicesReq, _ := http.NewRequest("DELETE", k.Addr+"/services/"+service, nil)

	_, err = http.DefaultClient.Do(servicesReq)

	if err != nil {
		fmt.Println(err)
	}

	return err
}

// AddRoutes add routes of service into kong
func (k *Kong) AddRoutes(serviceName string, methods []string, protocols []string, paths []string) (routesId string, err error) {

	v := make(url.Values)
	v.Add("service.name", serviceName)
	v["protocols[]"] = protocols
	v["methods[]"] = methods
	v["paths[]"] = paths
	v.Add("name", "routes-"+serviceName)

	resp, err := http.PostForm(k.Addr+"/routes/", v)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	return parseHttpRspWithAdd(resp)
}

// DelRoutes del routes of service from kong
func (k *Kong) DelRoutes(service string) (err error) {

	routesReq, _ := http.NewRequest("DELETE", k.Addr+"/services/"+service+"/routes/"+"routes-"+service, nil)

	_, err = http.DefaultClient.Do(routesReq)

	if err != nil {
		fmt.Println(err)
	}

	return err
}

func parseHttpRspWithAdd(resp *http.Response) (id string, err error) {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	fmt.Println(string(body))

	if resp.StatusCode != 201 {
		errRsp := &ErrRsp{}
		json.Unmarshal(body, errRsp)
		return "", errors.New(errRsp.Message)
	}

	nj, err := simplejson.NewJson(body)

	if err != nil {
		return "", err
	}
	id, _ = nj.Get("id").String()
	return id, nil
}