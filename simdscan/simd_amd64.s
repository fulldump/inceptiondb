#include "textflag.h"

// ============================================================================
// AVX2 SIMD constants: each character broadcast to all 32 bytes of a YMM
// ============================================================================

// " (0x22)
DATA const_quote<>+0x00(SB)/8, $0x2222222222222222
DATA const_quote<>+0x08(SB)/8, $0x2222222222222222
DATA const_quote<>+0x10(SB)/8, $0x2222222222222222
DATA const_quote<>+0x18(SB)/8, $0x2222222222222222
GLOBL const_quote<>(SB), (RODATA|NOPTR), $32

// \ (0x5C)
DATA const_backslash<>+0x00(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA const_backslash<>+0x08(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA const_backslash<>+0x10(SB)/8, $0x5c5c5c5c5c5c5c5c
DATA const_backslash<>+0x18(SB)/8, $0x5c5c5c5c5c5c5c5c
GLOBL const_backslash<>(SB), (RODATA|NOPTR), $32

// { (0x7B)
DATA const_lbrace<>+0x00(SB)/8, $0x7b7b7b7b7b7b7b7b
DATA const_lbrace<>+0x08(SB)/8, $0x7b7b7b7b7b7b7b7b
DATA const_lbrace<>+0x10(SB)/8, $0x7b7b7b7b7b7b7b7b
DATA const_lbrace<>+0x18(SB)/8, $0x7b7b7b7b7b7b7b7b
GLOBL const_lbrace<>(SB), (RODATA|NOPTR), $32

// } (0x7D)
DATA const_rbrace<>+0x00(SB)/8, $0x7d7d7d7d7d7d7d7d
DATA const_rbrace<>+0x08(SB)/8, $0x7d7d7d7d7d7d7d7d
DATA const_rbrace<>+0x10(SB)/8, $0x7d7d7d7d7d7d7d7d
DATA const_rbrace<>+0x18(SB)/8, $0x7d7d7d7d7d7d7d7d
GLOBL const_rbrace<>(SB), (RODATA|NOPTR), $32

// [ (0x5B)
DATA const_lbracket<>+0x00(SB)/8, $0x5b5b5b5b5b5b5b5b
DATA const_lbracket<>+0x08(SB)/8, $0x5b5b5b5b5b5b5b5b
DATA const_lbracket<>+0x10(SB)/8, $0x5b5b5b5b5b5b5b5b
DATA const_lbracket<>+0x18(SB)/8, $0x5b5b5b5b5b5b5b5b
GLOBL const_lbracket<>(SB), (RODATA|NOPTR), $32

// ] (0x5D)
DATA const_rbracket<>+0x00(SB)/8, $0x5d5d5d5d5d5d5d5d
DATA const_rbracket<>+0x08(SB)/8, $0x5d5d5d5d5d5d5d5d
DATA const_rbracket<>+0x10(SB)/8, $0x5d5d5d5d5d5d5d5d
DATA const_rbracket<>+0x18(SB)/8, $0x5d5d5d5d5d5d5d5d
GLOBL const_rbracket<>(SB), (RODATA|NOPTR), $32

// , (0x2C)
DATA const_comma<>+0x00(SB)/8, $0x2c2c2c2c2c2c2c2c
DATA const_comma<>+0x08(SB)/8, $0x2c2c2c2c2c2c2c2c
DATA const_comma<>+0x10(SB)/8, $0x2c2c2c2c2c2c2c2c
DATA const_comma<>+0x18(SB)/8, $0x2c2c2c2c2c2c2c2c
GLOBL const_comma<>(SB), (RODATA|NOPTR), $32

// : (0x3A)
DATA const_colon<>+0x00(SB)/8, $0x3a3a3a3a3a3a3a3a
DATA const_colon<>+0x08(SB)/8, $0x3a3a3a3a3a3a3a3a
DATA const_colon<>+0x10(SB)/8, $0x3a3a3a3a3a3a3a3a
DATA const_colon<>+0x18(SB)/8, $0x3a3a3a3a3a3a3a3a
GLOBL const_colon<>(SB), (RODATA|NOPTR), $32

// ============================================================================
// func classify32(data *byte) (quotes, backslashes, structural uint32)
//
// Classifies 32 bytes at *data using AVX2 parallel byte comparison.
// Returns three 32-bit bitmasks where bit i indicates data[i] matches.
// ============================================================================
TEXT ·classify32(SB), NOSPLIT, $0-20
	MOVQ data+0(FP), SI

	// Load 32 bytes from data
	VMOVDQU (SI), Y0

	// --- Quotes: compare all 32 bytes with '"' ---
	VMOVDQU const_quote<>(SB), Y1
	VPCMPEQB Y1, Y0, Y2
	VPMOVMSKB Y2, AX
	MOVL AX, quotes+8(FP)

	// --- Backslashes: compare all 32 bytes with '\' ---
	VMOVDQU const_backslash<>(SB), Y1
	VPCMPEQB Y1, Y0, Y2
	VPMOVMSKB Y2, AX
	MOVL AX, backslashes+12(FP)

	// --- Structural: { | } | [ | ] | , | : ---
	// Compare with '{' and start accumulating
	VMOVDQU const_lbrace<>(SB), Y1
	VPCMPEQB Y1, Y0, Y3

	// OR with '}'
	VMOVDQU const_rbrace<>(SB), Y1
	VPCMPEQB Y1, Y0, Y2
	VPOR Y2, Y3, Y3

	// OR with '['
	VMOVDQU const_lbracket<>(SB), Y1
	VPCMPEQB Y1, Y0, Y2
	VPOR Y2, Y3, Y3

	// OR with ']'
	VMOVDQU const_rbracket<>(SB), Y1
	VPCMPEQB Y1, Y0, Y2
	VPOR Y2, Y3, Y3

	// OR with ','
	VMOVDQU const_comma<>(SB), Y1
	VPCMPEQB Y1, Y0, Y2
	VPOR Y2, Y3, Y3

	// OR with ':'
	VMOVDQU const_colon<>(SB), Y1
	VPCMPEQB Y1, Y0, Y2
	VPOR Y2, Y3, Y3

	// Extract final structural bitmask
	VPMOVMSKB Y3, AX
	MOVL AX, structural+16(FP)

	// Clean up AVX state to avoid SSE transition penalty
	VZEROUPPER
	RET

// ============================================================================
// func hasAVX2() bool
//
// Uses CPUID leaf 7 to check for AVX2 support (EBX bit 5).
// ============================================================================
TEXT ·hasAVX2(SB), NOSPLIT, $0-1
	MOVL $7, AX
	XORL CX, CX
	CPUID
	SHRL $5, BX
	ANDL $1, BX
	MOVB BX, ret+0(FP)
	RET
