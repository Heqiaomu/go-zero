package discovery

import (
	"fmt"
	log "github.com/Heqiaomu/glog"
	"testing"
)

func TestFlow(t *testing.T) {
	SkipConvey(`test get service http addr`, t, func() {
		log.Logger()

		mysqlSvcer, err := NewEtcdSvcer([]string{"172.16.1.121:31433"}, "b20.service.mysql")
		ShouldBeNil(err)
		ShouldNotBeNil(mysqlSvcer)
		defer mysqlSvcer.Close()

		coreSvcer, err := NewEtcdSvcer([]string{"172.16.1.121:31433"}, "b20.service.core")
		ShouldBeNil(err)
		ShouldNotBeNil(coreSvcer)
		defer coreSvcer.Close()

		mysqlHttpAddr := mysqlSvcer.GetHttpAddr()
		ShouldNotBeEmpty(mysqlHttpAddr)
		fmt.Println(mysqlHttpAddr)

		coreHttpAddr := coreSvcer.GetHttpAddr()
		ShouldNotBeEmpty(coreHttpAddr)
		fmt.Println(coreHttpAddr)
	})
}
