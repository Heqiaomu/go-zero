package serverinterceptors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/zeromicro/go-zero/core/interceptor"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stat"
	"github.com/zeromicro/go-zero/core/syncx"
	"github.com/zeromicro/go-zero/core/timex"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

const defaultSlowThreshold = time.Millisecond * 500

var slowThreshold = syncx.ForAtomicDuration(defaultSlowThreshold)

// SetSlowThreshold sets the slow threshold.
func SetSlowThreshold(threshold time.Duration) {
	slowThreshold.Set(threshold)
}

// UnaryStatInterceptor returns a func that uses given metrics to report stats.
func UnaryStatInterceptor(metrics *stat.Metrics, s *interceptor.StatInterceptorConf) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer handleCrash(func(r interface{}) {
			err = toPanicError(r)
		})

		startTime := timex.Now()

		resp, err = handler(ctx, req)

		duration := timex.Since(startTime)
		metrics.Add(stat.Task{
			Duration: duration,
		})
		var ireq = req
		var iresp = resp
		if duration < slowThreshold.Load() && err == nil {
			method := info.FullMethod[strings.LastIndex(info.FullMethod, "/")+1:]
			if s.RequestFilter[method] {
				return
			}
			if s.RequestContentFilter[method] {
				ireq = "filtered"
			}
			if s.ResponseContentFilter[method] {
				iresp = "filtered"
			}
		}
		logDuration(ctx, info.FullMethod, ireq, err, iresp, duration)

		return
	}
}

func logDuration(ctx context.Context, method string, req interface{}, rpcerr error, resp interface{}, duration time.Duration) {
	var addr string
	client, ok := peer.FromContext(ctx)
	if ok {
		addr = client.Addr.String()
	}
	reqContext, err := json.Marshal(req)
	respContext, err := json.Marshal(resp)

	closeReqPrint := viper.GetBool("grpc.log.close_req_print")
	closeRespPrint := viper.GetBool("grpc.log.close_resp_print")
	ClosePrintSwitch := viper.GetBool("grpc.log.close_print")
	ContextSizeThreshold := viper.GetInt("grpc.log.context_size")
	if ContextSizeThreshold == 0 {
		ContextSizeThreshold = 4096
	}
	if len(string(reqContext)) > ContextSizeThreshold {
		reqContext = []byte(fmt.Sprintf("The request content is too long, exceeding the default length of %d. It is configured with grpc.log.context_size", ContextSizeThreshold))
	}
	if len(string(respContext)) > ContextSizeThreshold {
		respContext = []byte(fmt.Sprintf("The response content is too long, exceeding the default length of %d. It is configured with grpc.log.context_size", ContextSizeThreshold))
	}

	if closeReqPrint {
		reqContext = nil
	}
	if closeRespPrint {
		respContext = nil
	}

	if err != nil {
		logx.WithContext(ctx).Errorf("%s - %s", addr, err.Error())
	} else if duration > slowThreshold.Load() {
		if rpcerr != nil {
			logx.WithContext(ctx).WithDuration(duration).Slowf("[RPC] slowcall - %s - %s - %s - %s - %s",
				addr, method, string(reqContext), rpcerr.Error(), string(respContext))
		} else {
			logx.WithContext(ctx).WithDuration(duration).Slowf("[RPC] slowcall - %s - %s - %s - %s",
				addr, method, string(reqContext), string(respContext))
		}
	} else {
		if rpcerr != nil {
			logx.WithContext(ctx).WithDuration(duration).Infof("%s - %s - %s - %s - %s", addr, method, string(reqContext), rpcerr.Error(), string(respContext))
		} else {
			// if ClosePrintSwitch is true, close print
			if !ClosePrintSwitch {
				logx.WithContext(ctx).WithDuration(duration).Infof("%s - %s - %s - %s", addr, method, string(reqContext), string(respContext))
			}
		}
	}
}
