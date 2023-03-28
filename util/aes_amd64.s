// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build amd64,!gccgo

// func xor16(dst, a, b *byte)
TEXT ·xor16(SB),4,$0
	MOVQ dst+0(FP), AX
	MOVQ a+8(FP), BX
	MOVQ b+16(FP), CX
	MOVUPS 0(BX), X0
	MOVUPS 0(CX), X1
	PXOR X1, X0
	MOVUPS X0, 0(AX)
	RET

// func encryptAes128(xk *uint32, dst, src *byte)
TEXT ·encryptAes128(SB),4,$0
	MOVQ xk+0(FP), AX
	MOVQ dst+8(FP), DX
	MOVQ src+16(FP), BX
	MOVUPS 0(AX), X1
	MOVUPS 0(BX), X0
	ADDQ $16, AX
	PXOR X1, X0
	MOVUPS 0(AX), X1
	AESENC X1, X0
	MOVUPS 16(AX), X1
	AESENC X1, X0
	MOVUPS 32(AX), X1
	AESENC X1, X0
	MOVUPS 48(AX), X1
	AESENC X1, X0
	MOVUPS 64(AX), X1
	AESENC X1, X0
	MOVUPS 80(AX), X1
	AESENC X1, X0
	MOVUPS 96(AX), X1
	AESENC X1, X0
	MOVUPS 112(AX), X1
	AESENC X1, X0
	MOVUPS 128(AX), X1
	AESENC X1, X0
	MOVUPS 144(AX), X1
	AESENCLAST X1, X0
	MOVUPS X0, 0(DX)
	RET

// func aes128MMO(xk *uint32, dst, src *byte)
TEXT ·aes128MMO(SB),4,$0
	MOVQ xk+0(FP), AX
	MOVQ dst+8(FP), DX
	MOVQ src+16(FP), BX
	MOVUPS 0(AX), X1
	MOVUPS 0(BX), X0
	ADDQ $16, AX
	PXOR X1, X0
	MOVUPS 0(AX), X1
	AESENC X1, X0
	MOVUPS 16(AX), X1
	AESENC X1, X0
	MOVUPS 32(AX), X1
	AESENC X1, X0
	MOVUPS 48(AX), X1
	AESENC X1, X0
	MOVUPS 64(AX), X1
	AESENC X1, X0
	MOVUPS 80(AX), X1
	AESENC X1, X0
	MOVUPS 96(AX), X1
	AESENC X1, X0
	MOVUPS 112(AX), X1
	AESENC X1, X0
	MOVUPS 128(AX), X1
	AESENC X1, X0
	MOVUPS 144(AX), X1
	AESENCLAST X1, X0
	MOVUPS 0(BX), X1
	PXOR X1, X0
	MOVUPS X0, 0(DX)
	RET


// func expandKeyAsm(key *byte, enc *uint32) {
// Note that round keys are stored in uint128 format, not uint32
TEXT ·expandKeyAsm(SB),4,$0
	MOVQ key+0(FP), AX
	MOVQ enc+8(FP), BX
	MOVUPS (AX), X0
	// enc
	MOVUPS X0, (BX)
	ADDQ $16, BX
	PXOR X4, X4 // _expand_key_* expect X4 to be zero
	AESKEYGENASSIST $0x01, X0, X1
	CALL _expand_key_128<>(SB)
	AESKEYGENASSIST $0x02, X0, X1
	CALL _expand_key_128<>(SB)
	AESKEYGENASSIST $0x04, X0, X1
	CALL _expand_key_128<>(SB)
	AESKEYGENASSIST $0x08, X0, X1
	CALL _expand_key_128<>(SB)
	AESKEYGENASSIST $0x10, X0, X1
	CALL _expand_key_128<>(SB)
	AESKEYGENASSIST $0x20, X0, X1
	CALL _expand_key_128<>(SB)
	AESKEYGENASSIST $0x40, X0, X1
	CALL _expand_key_128<>(SB)
	AESKEYGENASSIST $0x80, X0, X1
	CALL _expand_key_128<>(SB)
	AESKEYGENASSIST $0x1b, X0, X1
	CALL _expand_key_128<>(SB)
	AESKEYGENASSIST $0x36, X0, X1
	CALL _expand_key_128<>(SB)
	RET

TEXT _expand_key_128<>(SB),4,$0
	PSHUFD $0xff, X1, X1
	SHUFPS $0x10, X0, X4
	PXOR X4, X0
	SHUFPS $0x8c, X0, X4
	PXOR X4, X0
	PXOR X1, X0
	MOVUPS X0, (BX)
	ADDQ $16, BX
	RET
