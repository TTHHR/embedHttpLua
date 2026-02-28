
#include <stdio.h>
#include <unistd.h>
#include <dlfcn.h>
#include "libembedhttplua.h"

int target_value = 100;

void avm_site_action(int new_value) {
    printf("avm_site_action called with value: %d\n", new_value);
    target_value = new_value;
}

int main() {

    void* handle = dlopen("../../libembedhttplua.so", RTLD_LAZY);
    if (!handle) {
        fprintf(stderr, "加载库失败: %s\n", dlerror());
        return 1;
    }

    // 3. 初始化 Go 逻辑
    typedef void (*InitFunc)();
    InitFunc init = (InitFunc)dlsym(handle, "InitLib");
    if (!init) {
        fprintf(stderr, "找不到 InitLib 函数\n");
        return 1;
    }
    
    init(); // 启动 Go 的 HTTP 服务

    while(1) {
        printf("Current target_value: %d \n", target_value);
        printf("funcA() = %d, funcB(3, 4) = %d\n", funcA(), funcB(3, 4));
        sleep(2); // 每2秒打印一次
    }

    return 0;
}