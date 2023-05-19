package apisix

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestApisix_AddRoutes(t *testing.T) {
	Convey("apisix", t, func() {
		//Convey("getroutes", func() {
		//	apisix := &Apisix{
		//		Addr:"http://apisix.develop.blocface.baas.hyperchain.cn",
		//	}
		//	id,err := apisix.GetRoutes("ggg")
		//	fmt.Println(id,err)
		//})
		//Convey("addroutes", func() {
		//	apisix := &Apisix{
		//		Addr:"http://apisix.develop.blocface.baas.hyperchain.cn",
		//	}
		//	id,err := apisix.AddRoutes("core","00000000000000000258")
		//	fmt.Println(id,err)
		//})
		//Convey("getupstream", func() {
		//	apisix := &Apisix{
		//		Addr:"http://apisix.develop.blocface.baas.hyperchain.cn",
		//	}
		//	ups,err := apisix.GetUpstreams("sdfd")
		//	fmt.Println(ups,err)
		//})
		//Convey("addupstream", func() {
		//	apisix := &Apisix{
		//		Addr:"http://apisix.develop.blocface.baas.hyperchain.cn",
		//	}
		//	ups,err := apisix.AddUpstreams("b20.service.core","blocface-core.blocface-develop.svc.cluster.local:8001",10)
		//	fmt.Println(ups,err)
		//})

		//Convey("addupstream", func() {
		//		apisix := &Apisix{
		//			Addr:"http://apisix.develop.blocface.baas.hyperchain.cn",
		//		}
		//		apisix.DelUpstreams("b20.service.core","172.16.1.121:31915")
		//	})
		//Convey("DelAllRoutes", func() {
		//	apisix := &Apisix{
		//		Addr:"http://apisix.develop.blocface.baas.hyperchain.cn",
		//	}
		//	err := apisix.DelAllRoutes()
		//	fmt.Println(err)
		//	err = apisix.DelAllUpstreams()
		//	fmt.Println(err)
		//})

		//Convey("new", func() {
		//	apisix := &Apisix{
		//		Addr:"http://apisix.develop.blocface.baas.hyperchain.cn",
		//	}
		//	ups,err := apisix.AddUpstreams("bbb","1.1.1.1:8080",10)
		//	apisix.AddRoutes("bbb",ups)
		//
		//	apisix.AddUpstreams("bbb","22.22.22.22:8080",10)
		//	fmt.Println(ups,err)
		//
		//	apisix.DelUpstreams("bbb","1.1.1.1:8080")
		//	apisix.DelUpstreams("bbb","22.22.22.22:8080")
		//})

		//Convey("delupstream", func() {
		//	apisix := &Apisix{
		//		Addr:"http://apisix.develop.blocface.baas.hyperchain.cn",
		//	}
		//	apisix.DelUpstreams("bbb","1.1.2.1:8080")
		//})
		//Convey("delroutes", func() {
		//	apisix := &Apisix{
		//		Addr:"http://apisix.develop.blocface.baas.hyperchain.cn",
		//	}
		//	apisix.DelRoutes("ttt")
		//})
	})
}
