#include "frida-handler.h"
#include <stdlib.h>
#include <stdio.h>

// 全局变量
static GumInterceptor * interceptor = NULL;
static gboolean is_gum_initialized = FALSE;

// 引用 Go 导出的回调
extern void go_on_enter_handler(uintptr_t addr, GumInvocationContext * ic);

// 辅助函数：Frida 的 enter 回调会调用这个 C 函数，进而调用 Go
static void on_enter_proxy (GumInvocationContext * ic, gpointer user_data) {
    // user_data 里存的就是我们要 Hook 的原始地址
    uintptr_t addr = (uintptr_t) user_data;
    go_on_enter_handler(addr, ic);
}

static void on_leave_proxy (GumInvocationContext * ic, gpointer user_data) {
    // 暂时用不到 leave
}

void init_gum() {
    if (is_gum_initialized) return;
    
    gum_init_embedded();
    interceptor = gum_interceptor_obtain();
    is_gum_initialized = TRUE;
    printf("[C] Frida-Gum initialized successfully.\n");
}

uintptr_t find_symbol(const char* name) {
    init_gum();
    // 官方示例使用的是 gum_module_find_global_export_by_name
    return (uintptr_t) gum_module_find_global_export_by_name(name);
}

int attach_hook(uintptr_t addr) {
    init_gum();
    
    // 使用官方示例的 gum_make_call_listener
    // user_data 传入地址本身，这样 enter 时就能知道是哪个函数触发的
    GumInvocationListener * listener = gum_make_call_listener(on_enter_proxy, on_leave_proxy, (gpointer)addr, NULL);

    gum_interceptor_begin_transaction(interceptor);
    GumAttachReturn res = gum_interceptor_attach(interceptor, (gpointer)addr, listener, (gpointer)addr, GUM_ATTACH_FLAGS_NONE);
    gum_interceptor_end_transaction(interceptor);

    // 注意：在实际生产中，你需要记录 listener 以便后续 g_object_unref
    // 这里简单处理，返回状态
    return (res == GUM_ATTACH_OK) ? 0 : -1;
}

void reset_all_hooks() {
    if (interceptor == NULL) return;
    
    printf("[C] Reverting all hooks...\n");
    // gum_interceptor_begin_transaction(interceptor);
    // gum_interceptor_revert(interceptor, NULL);
    // gum_interceptor_end_transaction(interceptor);
}