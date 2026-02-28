#ifndef FRIDA_HANDLER_H
#define FRIDA_HANDLER_H

#if defined(__cplusplus)
extern "C" {
#endif
#include "frida-gum.h"
#include <stdint.h>

void init_gum();
int attach_hook(uintptr_t addr);
void reset_all_hooks();
uintptr_t find_symbol(const char* name);
#if defined(__cplusplus)
}
#endif
#endif