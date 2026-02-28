print("------------------------------------")
local arch = get_arch()
print("当前检测到架构: " .. arch)

function quick_hook(func_name, return_val)
    local addr = get_sym_addr(func_name)
    if not addr then return end

    make_writable(addr, 16)
    
    if arch == "amd64" then

        write_mem(addr, string.char(0xB8))
        write_int32(addr + 1, return_val)
        write_mem(addr + 5, string.char(0xC3))
        
    elseif arch == "arm64" then
        local mov_w0 = 0x52800000 + (bit_lshift(return_val, 5))
        local ret = 0xD65F03C0
        
        write_int32(addr, mov_w0)
        write_int32(addr + 4, ret)
    end
    
    print("已成功 Hook [" .. func_name .. "] 返回值 -> " .. return_val)
end

function bit_lshift(val, shift)
    return val * (2 ^ shift)
end

quick_hook("funcA", 88)
quick_hook("funcB", 999)
local addr = get_sym_addr("target_value")
    if addr then 
		write_int32(addr, 15)
	end
print("------------------------------------")