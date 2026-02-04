// tsc_amd64.s
#include "textflag.h"

// 定义函数 GetTSC，返回 TSC 的值
TEXT ·GetTSC(SB), NOSPLIT, $0-8
    RDTSC       // 读取 TSC 寄存器的值到 EDX:EAX
    SHLQ $32, DX // 将 EDX 左移 32 位
    ADDQ DX, AX  // 将 EDX 和 EAX 合并到 RAX
    MOVQ AX, ret+0(FP) // 将结果存储到返回值
    RET
