package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc/balancer/roundrobin"
	"math/rand"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

type GrpcBaseEtcd struct {
	Conn *grpc.ClientConn
	Cli  *clientv3.Client
}

// CreateGrpcConn create a grpc conn
func Dial(endpoints []string, serviceName string) (ge *GrpcBaseEtcd, err error) {
	cli, err := clientv3.NewFromURLs(endpoints)
	if err != nil {
		return nil, err
	}

	r, _ := resolver.NewBuilder(cli)
	conn, err := grpc.Dial(fmt.Sprintf("etcd:///%s", serviceName),
		grpc.WithResolvers(r),
		grpc.WithBalancerName(roundrobin.Name),
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)

	if err != nil {
		return nil, err
	}

	return &GrpcBaseEtcd{Conn: conn, Cli: cli}, nil

}

// GetGrpcAddr get grpc addr with service name
func GetGrpcAddr(endpoints []string, serviceName string) (grpcAddr string, err error) {
	cli, err := clientv3.NewFromURLs(endpoints)
	if err != nil {
		return "", errors.Wrap(err, "fail to init etcd client")
	}
	defer cli.Close()

	// {"Addr":"172.16.1.124:32754","Metadata":"","HttpAddr":"blocface-chainctl.blocface-develop.svc.cluster.local:8001"}
	get, err := cli.Get(context.Background(), serviceName, clientv3.WithPrefix())
	if err != nil {
		return "", errors.Wrap(err, "fail to cli get")
	}
	if len(get.Kvs) == 0 {
		return "", fmt.Errorf("fail to get service(%s), because get service is empty", serviceName)
	}

	rand.Seed(time.Now().Unix())
	index := rand.Intn(len(get.Kvs))

	item := get.Kvs[index]

	json, err := simplejson.NewJson(item.Value)
	if err != nil {
		return "", errors.Wrap(err, "simplejson new")
	}
	grpcAddr, err = json.Get("Addr").String()
	if err != nil {
		return "", errors.Wrap(err, "get addr")
	}
	return

}

// GetHttpAddr get http addr with service name
func GetHttpAddr(endpoints []string, serviceName string) (httpAddr string, err error) {
	cli, err := clientv3.NewFromURLs(endpoints)
	if err != nil {
		return "", err
	}
	defer cli.Close()

	// {"Addr":"172.16.1.124:32754","Metadata":"","HttpAddr":"blocface-chainctl.blocface-develop.svc.cluster.local:8001"}
	get, err := cli.Get(context.Background(), serviceName, clientv3.WithPrefix())
	if err != nil {
		return "", err
	}
	if len(get.Kvs) == 0 {
		return "", fmt.Errorf("get no result with service(%s)", serviceName)
	}

	rand.Seed(time.Now().Unix())
	index := rand.Intn(len(get.Kvs))

	item := get.Kvs[index]
	json, err := simplejson.NewJson(item.Value)
	if err != nil {
		return "", fmt.Errorf("fail to parse json, err: [%v]", err)
	}
	httpAddr, err = json.Get("HttpAddr").String()
	if err != nil {
		return "", fmt.Errorf("fail to get httpAddr, err: [%v]", err)
	}
	return
}

// GetHttpAddr get http addrs with service name
func GetHttpAddrs(endpoints []string, serviceName string) ([]string, error) {
	cli, err := clientv3.NewFromURLs(endpoints)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	// {"Addr":"172.16.1.124:32754","Metadata":"","HttpAddr":"blocface-chainctl.blocface-develop.svc.cluster.local:8001"}
	get, err := cli.Get(context.Background(), serviceName, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	if len(get.Kvs) == 0 {
		return nil, fmt.Errorf("get no result with service(%s)", serviceName)
	}
	var httpAddrs []string

	for _, v := range get.Kvs {
		var m = map[string]interface{}{}
		err := json.Unmarshal(v.Value, &m)
		if err != nil {
			return nil, errors.Wrap(err, "Unmarshal registervalue")
		}
		httpAddrs = append(httpAddrs, m["HttpAddr"].(string))
	}
	if httpAddrs == nil {
		return nil, fmt.Errorf("get no http addr result with service(%s)", serviceName)
	}
	return httpAddrs, nil
}

func GetServiceInfo(endpoints []string, serviceName string) (string, error) {
	cli, err := clientv3.NewFromURLs(endpoints)
	if err != nil {
		return "", errors.Wrapf(err, "new etcd client by [%v]", endpoints)
	}
	defer cli.Close()

	get, err := cli.Get(context.Background(), serviceName, clientv3.WithPrefix())
	if err != nil {
		return "", errors.Wrapf(err, "get service in etcd by [%s]", serviceName)
	}
	if len(get.Kvs) == 0 {
		return "", fmt.Errorf("fail to get service, because there is no result with service [%s]", serviceName)
	}

	rand.Seed(time.Now().Unix())
	index := rand.Intn(len(get.Kvs))

	item := get.Kvs[index]
	return string(item.Value), nil
}
