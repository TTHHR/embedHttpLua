package main

/*
#cgo CFLAGS: -I./include
#cgo LDFLAGS: -L./lib -lfrida-gum -ldl -lm -lrt -lpthread
#include "frida-gum.h"
#include "frida-handler.h"
#include <stdlib.h>
*/
import "C"
import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"sync"
	"unsafe"

	lua "github.com/yuin/gopher-lua"
)

//go:embed index.html
var staticFiles embed.FS

var (
	L            *lua.LState
	mutex        sync.Mutex
	hookRegistry = make(map[uintptr]*lua.LFunction)
)

//export go_on_enter_handler
func go_on_enter_handler(addr uintptr, ic *C.GumInvocationContext) {
	mutex.Lock()
	defer mutex.Unlock()
	if cb, ok := hookRegistry[addr]; ok {
		// 调用 Lua 回调
		if err := L.CallByParam(lua.P{Fn: cb, NRet: 0, Protect: true}, lua.LNumber(addr)); err != nil {
			fmt.Printf("[Lua Error] %v\n", err)
		}
	}
}

// --- Lua 导出函数 ---

func luaHook(L *lua.LState) int {
	addr := uintptr(L.CheckNumber(1))
	cb := L.CheckFunction(2)

	mutex.Lock()
	hookRegistry[addr] = cb
	mutex.Unlock()

	res := C.attach_hook(C.uintptr_t(addr))
	L.Push(lua.LBool(res == 0))
	return 1
}

func luaGetSym(L *lua.LState) int {
	name := L.CheckString(1)
	cName := C.CString(name)
	fmt.Printf("Looking up symbol: %s\n", name)
	defer C.free(unsafe.Pointer(cName))

	addr := C.find_symbol(cName)
	fmt.Printf("Symbol lookup: %s -> 0x%lx\n", name, uintptr(addr))
	if addr == 0 {
		fmt.Printf("[Go] Symbol not found: %s\n", name)
		L.Push(lua.LNil)
	} else {
		fmt.Printf("[Go] Symbol found: %s -> 0x%x\n", name, uint64(addr))
		L.Push(lua.LNumber(uintptr(addr)))
	}
	return 1
}

// 内存读写扩展
func luaReadMem(L *lua.LState) int {
	addr := uintptr(L.CheckNumber(1))
	size := L.CheckInt(2)
	if size <= 0 {
		L.Push(lua.LString(""))
		return 1
	}
	data := C.GoBytes(unsafe.Pointer(addr), C.int(size))
	L.Push(lua.LString(data))
	return 1
}

func luaWriteInt32(L *lua.LState) int {
	addr := uintptr(L.CheckNumber(1))
	val := uint32(L.CheckNumber(2))
	ptr := (*uint32)(unsafe.Pointer(addr))
	*ptr = val
	return 0
}

func luaReadPtr(L *lua.LState) int {
	addr := uintptr(L.CheckNumber(1))
	val := *(*uintptr)(unsafe.Pointer(addr))
	L.Push(lua.LNumber(val))
	return 1
}

func resetAllHooks() {
	mutex.Lock()
	defer mutex.Unlock()

	// 1. 调用 C 层还原内存
	C.reset_all_hooks()

	// 2. 清空 Go 层的注册表
	hookRegistry = make(map[uintptr]*lua.LFunction)
	fmt.Println("[System] 所有 Hook 已撤销并还原")
}

// --- HTTP 服务 ---

func startHTTPServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		data, err := staticFiles.ReadFile("index.html")

		if err != nil {

			http.Error(w, "Internal Server Error: Missing static files", 500)

			return

		}

		w.Header().Set("Content-Type", "text/html")

		w.Write(data)

	})
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", 405)
			return
		}
		body, _ := io.ReadAll(r.Body)

		resetAllHooks() // 上传新脚本前清理环境

		if err := L.DoString(string(body)); err != nil {
			fmt.Fprintf(w, "Lua Error: %v", err)
		} else {
			fmt.Fprint(w, "脚本部署成功，Hook 已生效")
		}
	})
	fmt.Println("Control Server: http://localhost:1532/upload")
	http.ListenAndServe(":1532", nil)
}

//export InitLib
func InitLib() {
	C.init_gum()
	L = lua.NewState()

	// 注册 Lua 全局函数
	L.SetGlobal("hook", L.NewFunction(luaHook))
	L.SetGlobal("get_sym", L.NewFunction(luaGetSym))
	L.SetGlobal("read_mem", L.NewFunction(luaReadMem))
	L.SetGlobal("write_int32", L.NewFunction(luaWriteInt32))
	L.SetGlobal("read_ptr", L.NewFunction(luaReadPtr))

	go startHTTPServer()
}

func main() {
	// 保持进程存活（如果作为独立程序运行）
	// select {}
}
