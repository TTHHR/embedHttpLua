
-- 1. 查找目标函数地址
local target_name = "funcB"
local addr = get_sym(target_name)
print("lua started")
if addr and addr > 0 then
    print(string.format("[Lua] 找到目标函数 %s, 地址: 0x%X", target_name, addr))

    -- 2. 部署 Hook
    -- 注意：目前你的 Go 封装只传递了 addr 给 Lua
    local success = hook(addr, function(call_addr)
        print("[Lua] >>> Hook 触发！函数 " .. target_name .. " 被调用了")
    end)

    if success then
        print("[Lua] Hook 部署成功！")
    else
        print("[Lua] Hook 部署失败，请检查 Frida 状态")
    end
else
    print("[Lua] 错误：无法在进程中找到符号: " .. target_name)
    print("[Lua] 提示：请确保编译 C 程序时使用了 -rdynamic 参数")
end

local val_addr = get_sym("target_value")
if val_addr and val_addr > 0 then
    print("[Lua] 尝试写入 target_value")
    -- 确保这里调用的名字和 Go L.SetGlobal 注册的一致
    write_int32(val_addr, 999) 
    print("[Lua] 写入完成")
end