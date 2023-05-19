package apisix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type Apisix struct {
	Addr []string
}

type Plugin struct {
	ProxyRewrite *ProxyRewrite `json:"proxy-rewrite"`
}

type ProxyRewrite struct {
	RegexUri []string `json:"regex_uri"`
	Scheme   string   `json:"scheme"`
}

// AddRoutes add routes of service into apisix
func (a *Apisix) AddRoutes(serviceName string, upstreamId string) (routesID string, err error) {

	// 获取是否已经存在这个serviceName的routes，如果已经存在，直接返回已存在的id
	getID, err := a.GetRoutes(serviceName)
	if err != nil {
		return "", err
	}
	if getID != "" {
		return getID, nil
	}

	plugin := &Plugin{ProxyRewrite: &ProxyRewrite{
		RegexUri: []string{fmt.Sprintf("^/%s/(.*)$", serviceName), "/$1"},
		Scheme:   "http",
	}}
	j := simplejson.New()
	j.Set("methods", []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS", "HEAD"})
	j.Set("desc", serviceName+"-routes")
	j.Set("uri", "/"+serviceName+"/*")
	j.Set("upstream_id", upstreamId)
	j.Set("plugins", plugin)

	b, _ := j.MarshalJSON()

	body, err := a.HttpDo("POST", "/apisix/admin/routes", b)

	rj, err := simplejson.NewJson(body)
	if err != nil {
		return "", err
	}
	routesID, err = GetCreatedIndexByAdd(rj)
	return
}

func (a *Apisix) DelRoutes(serviceName string) error {
	routesID, err := a.GetRoutes(serviceName)
	if err != nil {
		return err
	}
	if routesID == "" {
		return nil
	}
	_, err = a.HttpDo("DELETE", "/apisix/admin/routes/"+routesID, nil)
	return err
}

// AddRoutes add routes of service from apisix
func (a *Apisix) GetRoutes(serviceName string) (routesID string, err error) {

	body, err := a.HttpDo("GET", "/apisix/admin/routes", nil)

	j, _ := simplejson.NewJson(body)
	size, _ := GetNodesLen(j)

	for i := 0; i < size; i++ {
		desc, _ := GetDesc(j, i)
		if desc == serviceName+"-routes" {
			routesID, err = GetCreatedIndexByGet(j, i)
			if err != nil {
				return "", err
			}
			break
		}
	}

	return
}

type Upstream struct {
	UpstreamID string
	Desc       string
	Service    string
	Nodes      map[string]int
}

// AddUpstreams add upstream of service into apisix
func (a *Apisix) AddUpstreams(serviceName string, target string, weight int) (upstreamID string, err error) {
	upstream, err := a.GetUpstreams(serviceName)
	if err != nil {
		return "", err
	}

	j := simplejson.New()
	j.Set("type", "roundrobin")
	j.Set("desc", serviceName+"-upstreams")
	j.Set("enable_websocket", true)

	if upstream == nil {
		t := simplejson.New()
		t.Set(target, weight)
		j.Set("nodes", t)

		b, _ := j.MarshalJSON()
		body, err := a.HttpDo("POST", "/apisix/admin/upstreams", b)
		if err != nil {
			return "", err
		}
		rj, _ := simplejson.NewJson(body)
		return GetCreatedIndexByAdd(rj)
	} else {
		t := simplejson.New()
		t.Set(target, weight)

		for k, v := range upstream.Nodes {
			t.Set(k, v)
		}
		j.Set("nodes", t)

		b, _ := j.MarshalJSON()
		_, err := a.HttpDo("PUT", "/apisix/admin/upstreams/"+upstream.UpstreamID, b)
		if err != nil {
			return "", err
		}
	}
	return upstream.UpstreamID, nil
}

// GetUpstreams get upstream of service from apisix
func (a *Apisix) GetUpstreams(serviceName string) (upstream *Upstream, err error) {

	body, err := a.HttpDo("GET", "/apisix/admin/upstreams", nil)
	if err != nil {
		return nil, err
	}

	j, _ := simplejson.NewJson(body)
	size, _ := GetNodesLen(j)

	upstream = &Upstream{Service: serviceName, Nodes: make(map[string]int, 0)}

	for i := 0; i < size; i++ {

		desc, err := GetDesc(j, i)
		if err != nil {
			return nil, err
		}
		if desc != serviceName+"-upstreams" {
			continue
		}
		upstream.Desc = desc
		upstream.UpstreamID, err = GetCreatedIndexByGet(j, i)
		if err != nil {
			return nil, err
		}
		m, err := GetUpstreamsMap(j, i)
		if err != nil {
			return nil, err
		}
		for k, v := range m {
			i, _ := strconv.Atoi(string(v.(json.Number)))
			upstream.Nodes[k] = i
		}
		break
	}
	if upstream.UpstreamID == "" {
		return nil, nil
	}
	return
}

func (a *Apisix) DelUpstreams(serviceName string, target string) error {
	upstream, err := a.GetUpstreams(serviceName)
	if err != nil {
		return err
	}
	delete(upstream.Nodes, target)
	if 0 == len(upstream.Nodes) {

		if err = a.DelRoutes(serviceName); err != nil {
			return err
		}

		_, err := a.HttpDo("DELETE", "/apisix/admin/upstreams/"+upstream.UpstreamID, nil)
		if err != nil {
			return err
		}
	} else {
		j := simplejson.New()
		j.Set("type", "roundrobin")
		j.Set("desc", serviceName+"-upstreams")
		j.Set("enable_websocket", true)

		t := simplejson.New()
		for k, v := range upstream.Nodes {
			t.Set(k, v)
		}
		j.Set("nodes", t)

		b, _ := j.MarshalJSON()
		_, err := a.HttpDo("PUT", "/apisix/admin/upstreams/"+upstream.UpstreamID, b)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Apisix) DelAllRoutes() (err error) {
	body, err := a.HttpDo("GET", "/apisix/admin/routes", nil)

	j, _ := simplejson.NewJson(body)

	size, _ := GetNodesLen(j)

	for i := 0; i < size; i++ {
		routesID, err := GetCreatedIndexByGet(j, i)
		if err != nil {
			return err
		}
		_, err = a.HttpDo("DELETE", "/apisix/admin/routes/"+routesID, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Apisix) DelAllUpstreams() (err error) {

	body, err := a.HttpDo("GET", "/apisix/admin/upstreams", nil)
	if err != nil {
		return err
	}

	j, _ := simplejson.NewJson(body)
	size, _ := GetNodesLen(j)

	for i := 0; i < size; i++ {

		upstreamID, err := GetCreatedIndexByGet(j, i)
		if err != nil {
			return err
		}
		_, err = a.HttpDo("DELETE", "/apisix/admin/upstreams/"+upstreamID, nil)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
	}
	return nil
}

func (a *Apisix) HttpDo(method string, path string, body []byte) (data []byte, err error) {
	var q *http.Request
	client := &http.Client{}
	var resp *http.Response
	for i := 0; i < len(a.Addr); i++ {
		q, err = http.NewRequest(method, a.Addr[i]+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		q.Header.Set("X-API-KEY", "edd1c9f034335f136f87ad84b625c8f1")

		resp, err = client.Do(q)

		if err == nil {
			break
		}
	}

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return data, nil

}

func GetCreatedIndexByAdd(rj *simplejson.Json) (string, error) {
	key, err := rj.Get("node").Get("key").String()
	if err != nil {
		return "", err
	}
	return strings.Split(key, "/")[3], nil
}

func GetCreatedIndexByGet(rj *simplejson.Json, i int) (string, error) {
	key, err := rj.Get("node").Get("nodes").GetIndex(i).Get("key").String()
	if err != nil {
		return "", err
	}
	return strings.Split(key, "/")[3], nil
}

func GetNodesLen(rj *simplejson.Json) (int, error) {
	if rj == nil {
		return 0, nil
	}
	node := rj.Get("node")
	if node == nil {
		return 0, nil
	}
	nodes := node.Get("nodes")
	if nodes == nil {
		return 0, nil
	}
	arr, err := nodes.Array()
	if err != nil {
		return 0, err
	}
	size := len(arr)
	return size, nil
}

func GetDesc(rj *simplejson.Json, i int) (string, error) {
	return rj.Get("node").Get("nodes").GetIndex(i).Get("value").Get("desc").String()
}

func GetUpstreamsMap(rj *simplejson.Json, i int) (map[string]interface{}, error) {
	return rj.Get("node").Get("nodes").GetIndex(i).Get("value").Get("nodes").Map()
}
