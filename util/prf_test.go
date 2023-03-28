package util

import (
	"fmt"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/holiman/uint256"

	//"golang.org/x/crypto/chacha20"
	chacha20poly1305 "golang.org/x/crypto/chacha20poly1305"
)

func TestUnsafePRF(t *testing.T) {
	rng := rand.New(rand.NewSource(103))
	add := uint64(0)
	start := time.Now()
	key := uint256.Int{rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64()}
	//aead, _ := chacha20poly1305.New(key.Bytes())

	for i := 0; i < 10000000; i++ {
		//add += PRFEval(&key, uint64(i), nonce, y)
		add += nonSafePRFEval(key.Uint64(), uint64(i))
	}
	duration := time.Since(start)
	fmt.Printf("Time: %v, add: %v\n", duration, add)
	fmt.Printf("Unsafe PRF Average time: %v\n", duration/10000000)
}

func TestChachaPRF(t *testing.T) {
	rng := rand.New(rand.NewSource(103))
	add := uint64(0)
	start := time.Now()
	key := uint256.Int{rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64()}
	//aead, _ := chacha20poly1305.New(key.Bytes())
	nonce := make([]byte, chacha20poly1305.NonceSize)
	y := make([]byte, 8)
	for i := 0; i < 10000000; i++ {
		add += PRFEval1(&key, uint64(i), nonce, y)
	}
	duration := time.Since(start)
	fmt.Printf("Time: %v, add: %v\n", duration, add)
	fmt.Printf("Standard Chacha20 Average time: %v\n", duration/10000000)
}

func TestUnoffciailChachaPRF(t *testing.T) {
	//rng := rand.New(rand.NewSource(103))
	add := uint64(0)
	start := time.Now()
	//key := uint256.Int{rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64()}
	//aead, _ := chacha20.New(key.Bytes())
	var (
		keyBytes PrfKey
	)
	rand.Read(keyBytes[:])
	//copy(keyBytes[:], key.Bytes())

	for i := 0; i < 10000000; i++ {
		add += PRFEval2(&keyBytes, uint64(i))
		if i == 0 {
			fmt.Printf("keybytes %x, First: %v\n", keyBytes, add)
		}
	}
	duration := time.Since(start)
	fmt.Printf("Time: %v, add: %v\n", duration, add)
	fmt.Printf("Standard Chacha20 Average time: %v\n", duration/10000000)
}

func TestUnoffciailChachaPRF128(t *testing.T) {
	//rng := rand.New(rand.NewSource(103))
	add := uint64(0)
	start := time.Now()
	//key := uint256.Int{rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64()}
	//aead, _ := chacha20.New(key.Bytes())
	var (
		keyBytes PrfKey128
	)
	rand.Read(keyBytes[:])
	//copy(keyBytes[:], key.Bytes())

	for i := 0; i < 10000000; i++ {
		add += PRFEval3(&keyBytes, uint64(i))
		if i == 0 {
			fmt.Printf("keybytes %x, First: %v\n", keyBytes, add)
		}
	}
	duration := time.Since(start)
	fmt.Printf("Time: %v, add: %v\n", duration, add)
	fmt.Printf("Standard Chacha20 Average time: %v\n", duration/10000000)
}

func TestAESPRF128(t *testing.T) {
	//rng := rand.New(rand.NewSource(103))
	add := uint64(0)
	start := time.Now()
	//key := uint256.Int{rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64()}
	//aead, _ := chacha20.New(key.Bytes())
	var (
		keyBytes PrfKey128
	)
	rand.Read(keyBytes[:])
	//copy(keyBytes[:], key.Bytes())

	log.Printf("PRF Eval 4: %v\n", PRFEval4(&keyBytes, 0))
	log.Printf("PRF Eval 4: %v\n", PRFEval4(&keyBytes, 1))
	log.Printf("PRF Eval 4: %v\n", PRFEval4(&keyBytes, 2))
	log.Printf("PRF Eval 4: %v\n", PRFEval4(&keyBytes, 3))

	for i := 0; i < 10000000; i++ {
		add += PRFEval4(&keyBytes, uint64(i))
		if i == 0 {
			fmt.Printf("keybytes %x, First: %v\n", keyBytes, add)
		}
	}
	duration := time.Since(start)
	fmt.Printf("Time: %v, add: %v\n", duration, add)
	fmt.Printf("AES Average time: %v\n", duration/10000000)
}

func testDebugAES(test *testing.T) {
	var prfkeyL = []byte{36, 156, 50, 234, 92, 230, 49, 9, 174, 170, 205, 160, 98, 236, 29, 243}
	var prfkeyR = []byte{209, 12, 199, 173, 29, 74, 44, 128, 194, 224, 14, 44, 2, 201, 110, 28}
	var keyL = make([]uint32, 11*4)
	var keyR = make([]uint32, 11*4)

	expandKeyAsm(&prfkeyL[0], &keyL[0])
	expandKeyAsm(&prfkeyR[0], &keyR[0])
	fmt.Printf("%+v\n", keyL)
	fmt.Printf("%+v\n", keyR)

	seed := []byte{123, 56, 5, 24, 9, 20, 4, 9, 14, 10, 25, 10, 9, 26, 29, 43}
	s0 := new(block)
	aes128MMO(&keyL[0], &s0[0], &seed[0])
	fmt.Printf("%+v\n", seed)
	fmt.Printf("%+v\n", *s0)

	xor16(&s0[0], &s0[0], &s0[0])
	fmt.Printf("%+v\n", s0)
}

func main() {
	//testUnsafePRF()
	//testChachaPRF()
	//TestUnoffciailChachaPRF()
	/*
		if pass == "" {
			a := make([]byte, 32)
			copy(key[:32], a[:32])
			aead, _ = chacha20poly1305.NewX(a)
		}
		if msg == "" {
			a := make([]byte, 32)
			msg = string(a)

		}
	*/
}
