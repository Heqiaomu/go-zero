package clientinterceptors

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/spf13/viper"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/syncx"
	"github.com/zeromicro/go-zero/core/timex"
	"google.golang.org/grpc"
)

const defaultSlowThreshold = time.Millisecond * 500

var slowThreshold = syncx.ForAtomicDuration(defaultSlowThreshold)

// DurationInterceptor is an interceptor that logs the processing time.
func DurationInterceptor(ctx context.Context, method string, req, reply interface{},
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	serverName := path.Join(cc.Target(), method)
	start := timex.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)

	ContextSizeThreshold := viper.GetInt("grpc.log.client_context_size")
	if ContextSizeThreshold == 0 {
		ContextSizeThreshold = 4096
	}
	marshalReq, _ := json.Marshal(req)
	if len(string(marshalReq)) > ContextSizeThreshold {
		req = fmt.Sprintf("The request content is too long, exceeding the default length of %d. It is configured with grpc.log.client_context_size", ContextSizeThreshold)
	}
	marshalResp, _ := json.Marshal(reply)
	if len(marshalResp) > ContextSizeThreshold {
		reply = fmt.Sprintf("The response content is too long, exceeding the default length of %d. It is configured with grpc.log.client_context_size", ContextSizeThreshold)
	}

	if err != nil {
		logx.WithContext(ctx).WithDuration(timex.Since(start)).Infof("fail - %s - %v - %s",
			serverName, req, err.Error())
	} else {
		elapsed := timex.Since(start)
		if elapsed > slowThreshold.Load() {
			logx.WithContext(ctx).WithDuration(elapsed).Slowf("[RPC] ok - slowcall - %s - %v - %v",
				serverName, req, reply)
		}
	}

	return err
}

// SetSlowThreshold sets the slow threshold.
func SetSlowThreshold(threshold time.Duration) {
	slowThreshold.Set(threshold)
}
