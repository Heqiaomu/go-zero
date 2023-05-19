package serverinterceptors

import (
	"context"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryTimeoutInterceptor returns a func that sets timeout to incoming unary requests.
func UnaryTimeoutInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		var cancel context.CancelFunc

		// 配置特定接口的超时规则（单位：秒）
		apiTimeouts := viper.GetStringSlice("grpc.api.timeout")
		if ok, newTimeout := keyExist(info.FullMethod, apiTimeouts); ok {
			ctx, cancel = context.WithTimeout(ctx, time.Duration(newTimeout)*time.Second)
		} else {
			// 默认全局超时
			ctx, cancel = context.WithTimeout(ctx, timeout)
		}
		defer cancel()

		var resp interface{}
		var err error
		var lock sync.Mutex
		done := make(chan struct{})
		// create channel with buffer size 1 to avoid goroutine leak
		panicChan := make(chan interface{}, 1)
		go func() {
			defer func() {
				if p := recover(); p != nil {
					// attach call stack to avoid missing in different goroutine
					panicChan <- fmt.Sprintf("%+v\n\n%s", p, strings.TrimSpace(string(debug.Stack())))
				}
			}()

			lock.Lock()
			defer lock.Unlock()
			resp, err = handler(ctx, req)
			close(done)
		}()

		select {
		case p := <-panicChan:
			panic(p)
		case <-done:
			lock.Lock()
			defer lock.Unlock()
			return resp, err
		case <-ctx.Done():
			err := ctx.Err()

			if err == context.Canceled {
				err = status.Error(codes.Canceled, err.Error())
			} else if err == context.DeadlineExceeded {
				err = status.Error(codes.DeadlineExceeded, err.Error())
			}
			return nil, err
		}
	}
}

const (
	separate       = "="
	defaultTimeout = 60
)

func keyExist(FullMethod string, keys []string) (bool, int) {
	for _, key := range keys {
		var apiMethod string
		apiTimeout := defaultTimeout
		keySlice := strings.Split(key, separate)
		if len(keySlice) > 0 {
			apiMethod = keySlice[0]
		}
		if len(keySlice) == 2 {
			t, err := strconv.Atoi(keySlice[1])
			if err == nil {
				apiTimeout = t
			}
		}
		if FullMethod == apiMethod {
			return true, apiTimeout
		}
	}
	return false, defaultTimeout
}
