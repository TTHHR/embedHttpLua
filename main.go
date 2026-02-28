package main

import (
	"embed"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync" // 引入锁
	"time"
	"unsafe"

	lua "github.com/yuin/gopher-lua"
)

/*
#include <dlfcn.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <unistd.h>
#include <string.h>
#include <stdint.h>
#include <stdio.h>
typedef void (*func_int_t)(int);
// 读取 /proc/self/maps 获取第一个加载段的基址
static uintptr_t get_main_module_base() {
    FILE* f = fopen("/proc/self/maps", "r");
    if (!f) return 0;
    char line[256];
    uintptr_t base = 0;
    if (fgets(line, sizeof(line), f)) {
        sscanf(line, "%lx-", &base);
    }
    fclose(f);
    return base;
}
// 包装函数：将地址转为函数指针并执行
static void call_c_func_int(void* addr, int arg) {
    func_int_t f = (func_int_t)addr;
    f(arg);
}
*/
import "C"

//go:embed index.html
var staticFiles embed.FS

var (
	L     *lua.LState
	mutex sync.Mutex // 保证操作的原子性
)

// --- 内存备份管理 ---

type memBackup struct {
	addr uintptr
	data []byte
}

// 记录所有被修改过的内存原始值
var globalBackups []memBackup

// resetMemory 还原所有之前的内存修改
func resetMemory() {
	// 从后往前还原，处理重叠修改的情况
	for i := len(globalBackups) - 1; i >= 0; i-- {
		b := globalBackups[i]
		ptr := unsafe.Pointer(b.addr)
		for j := 0; j < len(b.data); j++ {
			*(*byte)(unsafe.Pointer(uintptr(ptr) + uintptr(j))) = b.data[j]
		}
	}
	globalBackups = nil
}

// --- Lua 导出函数扩展 ---
func luaGetModuleBase(L *lua.LState) int {
	base := uintptr(C.get_main_module_base())
	L.Push(lua.LNumber(base))
	return 1
}

// luaReadMemory(address, size)
func luaReadMemory(L *lua.LState) int {
	// 将 Lua 的数字转换为 uintptr
	addr := uintptr(L.CheckNumber(1))
	size := L.CheckInt(2)

	// 安全检查：防止读取长度为 0 或负数
	if size <= 0 {
		L.Push(lua.LString(""))
		return 1
	}

	// 将地址转换为 byte 切片读取
	ptr := unsafe.Pointer(addr)
	data := C.GoBytes(ptr, C.int(size))

	L.Push(lua.LString(data))
	return 1
}

// luaWriteMemory(address, data_string)
func luaWriteMemory(L *lua.LState) int {
	addr := uintptr(L.CheckNumber(1))
	data := L.CheckString(2)

	ptr := unsafe.Pointer(addr)
	src := []byte(data)

	original := C.GoBytes(ptr, C.int(len(src)))
	globalBackups = append(globalBackups, memBackup{addr: addr, data: original})

	// 循环写入字节
	for i := 0; i < len(src); i++ {
		*(*byte)(unsafe.Pointer(uintptr(ptr) + uintptr(i))) = src[i]
	}
	return 0
}

// luaGetSymbolAddr(name) -> address_number
func luaGetSymbolAddr(L *lua.LState) int {
	name := L.CheckString(1)
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	// RTLD_DEFAULT 在 Linux 下通常是 (void*)0
	// 它会搜索主程序及所有已加载库的符号
	addr := C.dlsym(nil, cName)

	if addr == nil {
		// 如果没找到，返回 nil 给 Lua，或者抛出错误
		L.Push(lua.LNil)
		return 1
	}

	L.Push(lua.LNumber(uintptr(addr)))
	return 1
}

// luaMakeWritable(address, size)
func luaMakeWritable(L *lua.LState) int {
	addr := uintptr(L.CheckNumber(1))

	// 获取页面大小并对齐地址
	pageSize := uintptr(C.sysconf(C._SC_PAGESIZE))
	pageAddr := addr & ^(pageSize - 1)

	// 设置内存为 可读、可写、可执行 (PROT_READ | PROT_WRITE | PROT_EXEC)
	res := C.mprotect(unsafe.Pointer(pageAddr), C.size_t(pageSize*2), C.PROT_READ|C.PROT_WRITE|C.PROT_EXEC)

	L.Push(lua.LBool(res == 0))
	return 1
}

// 获取当前系统架构，方便 Lua 切换 Hook 载荷
func luaGetArch(L *lua.LState) int {
	L.Push(lua.LString(runtime.GOARCH))
	return 1
}

// 直接写入一个 32 位整数（小端序）
func luaWriteInt32(L *lua.LState) int {
	addr := uintptr(L.CheckNumber(1))
	val := uint32(L.CheckNumber(2))

	ptr := unsafe.Pointer(addr)
	original := C.GoBytes(ptr, 4)
	globalBackups = append(globalBackups, memBackup{addr: addr, data: original})

	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, val)

	for i := 0; i < 4; i++ {
		*(*byte)(unsafe.Pointer(uintptr(ptr) + uintptr(i))) = buf[i]
	}
	return 0
}

// luaCallC(address, value)
func luaCallC(L *lua.LState) int {
	addr := uintptr(L.CheckNumber(1))
	arg := L.CheckInt(2)

	// 调用 C 包装函数
	C.call_c_func_int(unsafe.Pointer(addr), C.int(arg))

	return 0 // 无返回值给 Lua
}

// --- HTTP 服务逻辑 ---

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
			http.Error(w, "仅支持 POST 请求", http.StatusMethodNotAllowed)
			return
		}

		content, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, "读取失败: %v", err)
			return
		}
		defer r.Body.Close()

		// 【关键】使用锁并重置环境
		mutex.Lock()
		defer mutex.Unlock()

		// 1. 恢复之前脚本改动的所有内存逻辑
		resetMemory()
		if err := L.DoString(string(content)); err != nil {
			errMsg := fmt.Sprintf("Lua 错误: %v", err)
			fmt.Println(errMsg)
			fmt.Fprintf(w, errMsg)
			return
		}

		fmt.Fprintf(w, "旧脚本已撤销，新脚本执行成功！时间: %s", time.Now().Format("15:04:05"))
	})

	fmt.Println("HTTP 控制台已启动: http://localhost:1532")
	http.ListenAndServe(":1532", nil)
}

//export InitLib
func InitLib() {
	L = lua.NewState()
	// 注册 Lua 全局函数，供脚本调用
	L.SetGlobal("get_module_base", L.NewFunction(luaGetModuleBase))
	L.SetGlobal("read_mem", L.NewFunction(luaReadMemory))
	L.SetGlobal("write_mem", L.NewFunction(luaWriteMemory))
	L.SetGlobal("get_sym_addr", L.NewFunction(luaGetSymbolAddr))
	L.SetGlobal("make_writable", L.NewFunction(luaMakeWritable))
	L.SetGlobal("get_arch", L.NewFunction(luaGetArch))
	L.SetGlobal("write_int32", L.NewFunction(luaWriteInt32))
	L.SetGlobal("call_c", L.NewFunction(luaCallC))

	go startHTTPServer()
}

func main() {
	// 主函数留空，作为 .so 时不会被直接运行
}
