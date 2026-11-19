package app

import (
	"fmt"
	"sync/atomic"

	"wood-passage-creator/internal/port"
)

// Runtime 保存应用层共享的进程级依赖。
// 它在启动阶段初始化一次；当前事务等请求级状态仍由 context 承载。
type Runtime struct {
	port.TxManager
}

var currentRuntime atomic.Pointer[Runtime]

// InitRuntime 初始化全局 Runtime。重复初始化通常表示启动装配有误。
func InitRuntime(runtime Runtime) error {
	if runtime.TxManager == nil {
		return fmt.Errorf("app: nil transaction manager")
	}
	if !currentRuntime.CompareAndSwap(nil, &runtime) {
		return fmt.Errorf("app: runtime already initialized")
	}
	return nil
}

// CurrentRuntime 返回全局 Runtime；未初始化属于启动编程错误。
func CurrentRuntime() *Runtime {
	runtime := currentRuntime.Load()
	if runtime == nil {
		panic("app: runtime not initialized")
	}
	return runtime
}
