-- 1. 查找目标函数地址
local target_name = "funcB"
local addr = get_sym(target_name)

if addr and addr > 0 then
    print(string.format("[Lua] 找到目标函数 %s, 地址: 0x%X", target_name, addr))

    -- 2. 部署 Hook
    local success = hook(addr, function(ic, call_addr)
        print(string.format("[Lua] >>> Hook 触发！函数地址: 0x%X", call_addr))

        -- 读取 funcB(int a, int b) 的参数
        -- 索引通常从 0 开始 (0: 第一个参数, 1: 第二个参数)
        local arg0 = get_arg(ic, 0)
        local arg1 = get_arg(ic, 1)
        
        print(string.format("[Lua] 原始参数: a = %d, b = %d", arg0, arg1))

        -- 修改参数：例如将 a 修改为 888，将 b 翻倍
        local new_a = 888
        local new_b = arg1 * 2
        
        set_arg(ic, 0, new_a)
        set_arg(ic, 1, new_b)
        
        print(string.format("[Lua] 参数已篡改: a -> %d, b -> %d", new_a, new_b))
    end)

    if success then
        print("[Lua] Hook 部署成功！")
    else
        print("[Lua] Hook 部署失败")
    end
else
    print("[Lua] 错误：无法找到符号: " .. target_name)
end

-- 内存直接写入测试
local val_addr = get_sym("target_value")
if val_addr and val_addr > 0 then
    print(string.format("[Lua] 尝试写入 target_value @ 0x%X", val_addr))
    write_int32(val_addr, 999) 
    print("[Lua] 内存写入完成")
end