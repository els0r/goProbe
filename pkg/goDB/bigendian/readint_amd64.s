/////////////////////////////////////////////////////////////////////////////////
//
// readint_amd64.s
//
// Assembler implementation for the amd64 architecture
//
// Written by Lorenz Breidenbach lob@open.ch, November 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

# include "textflag.h"

// func ReadUint64At(b []byte, idx int) uint64
TEXT ·ReadUint64At(SB),NOSPLIT,$0-40
    MOVQ    b_cap+16(FP), AX  // AX = cap(b)
    MOVQ    idx+24(FP), BX    // BX = idx
    SHLQ    $3, BX            // BX = BX * 8 == idx * 8
    LEAQ    +8(BX), CX        // CX = BX + 8 == idx * 8 + 8
    CMPQ    AX, CX            // compare to CX
    JCS     panic             // if less then idx * 8 + 8, panic
    MOVQ    b+0(FP), CX       // CX = &b[0]
    ADDQ    BX, CX            // CX = CX + BX == &b[idx*8]
    MOVQ    (CX), CX          // CX = *CX == b[idx*8]
    BSWAPQ  CX                // convert from big endian to little endian
    MOVQ    CX, ret+32(FP)    // store result.
                              // 24 = 3 * 8 byte for slice + 8 bytes for idx
    RET
panic:
    CALL    runtime·panicindex(SB)

// func ReadInt64At(b []byte, idx int) int64
TEXT ·ReadInt64At(SB),NOSPLIT,$0-40
    MOVQ    b_cap+16(FP), AX  // AX = cap(b)
    MOVQ    idx+24(FP), BX    // BX = idx
    SHLQ    $3, BX            // BX = BX * 8 == idx * 8
    LEAQ    +8(BX), CX        // CX = BX + 8 == idx * 8 + 8
    CMPQ    AX, CX            // compare to CX
    JCS     panic             // if less then idx * 8 + 8, panic
    MOVQ    b+0(FP), CX       // CX = &b[0]
    ADDQ    BX, CX            // CX = CX + BX == &b[idx*8]
    MOVQ    (CX), CX          // CX = *CX == b[idx*8]
    BSWAPQ  CX                // convert from big endian to little endian
    MOVQ    CX, ret+32(FP)    // store result.
                              // 24 = 3 * 8 byte for slice + 8 bytes for idx
    RET
panic:
    CALL    runtime·panicindex(SB)

// func UnsafeReadUint64At(b []byte, idx int) uint64
TEXT ·UnsafeReadUint64At(SB),NOSPLIT,$0-40
    MOVQ    idx+24(FP), BX    // BX = idx
    SHLQ    $3, BX            // BX = BX * 8 == idx * 8
    MOVQ    b+0(FP), CX       // CX = &b[0]
    ADDQ    BX, CX            // CX = CX + BX == &b[idx*8]
    MOVQ    (CX), CX          // CX = *CX == b[idx*8]
    BSWAPQ  CX                // convert from big endian to little endian
    MOVQ    CX, ret+32(FP)    // store result.
                              // 24 = 3 * 8 byte for slice + 8 bytes for idx
    RET

// func UnsafeReadInt64At(b []byte, idx int) int64
TEXT ·UnsafeReadInt64At(SB),NOSPLIT,$0-40
    MOVQ    idx+24(FP), BX    // BX = idx
    SHLQ    $3, BX            // BX = BX * 8 == idx * 8
    MOVQ    b+0(FP), CX       // CX = &b[0]
    ADDQ    BX, CX            // CX = CX + BX == &b[idx*8]
    MOVQ    (CX), CX          // CX = *CX == b[idx*8]
    BSWAPQ  CX                // convert from big endian to little endian
    MOVQ    CX, ret+32(FP)    // store result.
                              // 24 = 3 * 8 byte for slice + 8 bytes for idx
    RET
