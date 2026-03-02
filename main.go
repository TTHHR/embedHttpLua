package main

/*
#cgo CFLAGS: -I./include
#include "frida-gum.h"
#include "frida-handler.h"
#include <stdlib.h>
#ifdef __ANDROID__
#include <android/log.h>
#endif

static inline void device_log(int level, const char* tag, const char* msg) {
#ifdef __ANDROID__
    __android_log_print(level, tag, "%s", msg);
#else
    printf("[%s] %s\n", tag, msg);
#endif
}
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
		ud := L.NewUserData()
		ud.Value = uintptr(unsafe.Pointer(ic))

		// 调用 Lua: cb(ic, addr)
		if err := L.CallByParam(lua.P{Fn: cb, NRet: 0, Protect: true}, ud, lua.LNumber(addr)); err != nil {
			fmt.Printf("[Lua Error] %v\n", err)
		}
	}
}

const (
	LogDebug = 3
	LogInfo  = 4
	LogError = 6
)

func LogToDevice(level int, tag string, msg string) {
	cTag := C.CString(tag)
	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cTag))
	defer C.free(unsafe.Pointer(cMsg))
	C.device_log(C.int(level), cTag, cMsg)
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
	var addr C.uintptr_t
	var name string
	var libName string

	top := L.GetTop()

	if top >= 2 {
		// 模式 A: get_sym("libavm.so", "func_name")
		libName = L.CheckString(1)
		name = L.CheckString(2)

		cLibName := C.CString(libName)
		cName := C.CString(name)
		defer C.free(unsafe.Pointer(cLibName))
		defer C.free(unsafe.Pointer(cName))

		addr = C.find_lib_symbol(cLibName, cName)
	} else {
		// 模式 B: get_sym("func_name") - 全局查找
		name = L.CheckString(1)
		cName := C.CString(name)
		defer C.free(unsafe.Pointer(cName))
		addr = C.find_symbol(cName)
	}

	// 日志输出
	displayTag := name
	if libName != "" {
		displayTag = fmt.Sprintf("[%s]%s", libName, name)
	}

	if addr == 0 {
		LogToDevice(LogError, "Symbol Lookup", fmt.Sprintf("Failed: %s", displayTag))
		L.Push(lua.LNil)
	} else {
		LogToDevice(LogInfo, "Symbol Lookup", fmt.Sprintf("Success: %s -> 0x%x", displayTag, uint64(addr)))
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
func luaGetArg(L *lua.LState) int {
	ic := (*C.GumInvocationContext)(unsafe.Pointer(L.CheckUserData(1).Value.(uintptr)))
	index := L.CheckInt(2)
	// 调用 Frida-Gum API 获取参数
	val := C.gum_invocation_context_get_nth_argument(ic, C.uint(index))
	L.Push(lua.LNumber(uintptr(val)))
	return 1
}

func luaReplaceArg(L *lua.LState) int {
	ic := (*C.GumInvocationContext)(unsafe.Pointer(L.CheckUserData(1).Value.(uintptr)))
	index := L.CheckInt(2)
	newVal := uintptr(L.CheckNumber(3))
	C.gum_invocation_context_replace_nth_argument(ic, C.guint(index), C.gpointer(unsafe.Pointer(newVal)))
	return 0
}

func luaReplaceRet(L *lua.LState) int {
	ic := (*C.GumInvocationContext)(unsafe.Pointer(L.CheckUserData(1).Value.(uintptr)))
	newRet := uintptr(L.CheckNumber(2))
	C.gum_invocation_context_replace_return_value(ic, C.gpointer(unsafe.Pointer(newRet)))
	return 0
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
	LogToDevice(LogInfo, "Control Server", "http://localhost:1532/upload")
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
	L.SetGlobal("get_arg", L.NewFunction(luaGetArg))
	L.SetGlobal("set_arg", L.NewFunction(luaReplaceArg))
	L.SetGlobal("set_ret", L.NewFunction(luaReplaceRet))
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		arg := L.CheckAny(1)
		LogToDevice(LogInfo, "LuaEngine", arg.String())
		return 0
	}))
	go startHTTPServer()
}

func main() {
	// 保持进程存活（如果作为独立程序运行）
	// select {}
}
