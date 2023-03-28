package util

import (
	//"crypto/sha256"
	"hash/fnv"
	"math"

	// "fmt"
	//////////////////"io"
	//"crypto/aes"
	//"crypto/cipher"

	"encoding/binary"
	rand "math/rand"

	uint256 "github.com/holiman/uint256"
	chacha20 "gitlab.com/yawning/chacha20.git"
	chacha20poly1305 "golang.org/x/crypto/chacha20poly1305"
)

const (
	DBEntrySize   = 8 // has to be a multiple of 8!!
	DBEntryLength = DBEntrySize / 8
)

type PrfKey256 [32]byte
type PrfNonce [12]byte
type PrfKey128 [16]byte
type block [16]byte
type DBEntry [DBEntryLength]uint64

type PrfKey PrfKey128

func RandKey256(rng *rand.Rand) PrfKey256 {
	var key [32]byte
	binary.LittleEndian.PutUint64(key[0:8], rng.Uint64())
	binary.LittleEndian.PutUint64(key[8:16], rng.Uint64())
	binary.LittleEndian.PutUint64(key[16:24], rng.Uint64())
	binary.LittleEndian.PutUint64(key[24:32], rng.Uint64())
	return key
}

func RandKey128(rng *rand.Rand) PrfKey128 {
	var key [16]byte
	binary.LittleEndian.PutUint64(key[0:8], rng.Uint64())
	binary.LittleEndian.PutUint64(key[8:16], rng.Uint64())
	return key
}

// now we have a 128-bit key
func RandKey(rng *rand.Rand) PrfKey {
	return PrfKey(RandKey128(rng))
}

// now we use the optimized aes to do the PRF
func PRFEval(key *PrfKey, x uint64) uint64 {
	return PRFEval4((*PrfKey128)(key), x)
}

func DBEntryXor(dst *DBEntry, src *DBEntry) {
	for i := 0; i < DBEntryLength; i++ {
		(*dst)[i] ^= (*src)[i]
	}
}

func DBEntryXorFromRaw(dst *DBEntry, src []uint64) {
	for i := 0; i < DBEntryLength; i++ {
		(*dst)[i] ^= src[i]
	}
}

func EntryIsEqual(a *DBEntry, b *DBEntry) bool {
	for i := 0; i < DBEntryLength; i++ {
		if (*a)[i] != (*b)[i] {
			return false
		}
	}
	return true
}

func RandDBEntry(rng *rand.Rand) DBEntry {
	var entry DBEntry
	for i := 0; i < DBEntryLength; i++ {
		entry[i] = rng.Uint64()
	}
	return entry
}

func GenDBEntry(key uint64, id uint64) DBEntry {
	var entry DBEntry
	for i := 0; i < DBEntryLength; i++ {
		entry[i] = DefaultHash((key ^ id) + uint64(i))
	}
	return entry
}

func ZeroEntry() DBEntry {
	ret := DBEntry{}
	for i := 0; i < DBEntryLength; i++ {
		ret[i] = 0
	}
	return ret
}

func DBEntryFromSlice(s []uint64) DBEntry {
	var entry DBEntry
	for i := 0; i < DBEntryLength; i++ {
		entry[i] = s[i]
	}
	return entry
}

// return ChunkSize, SetSize
func GenParams(DBSize uint64) (uint64, uint64) {
	// the chunksize is 2*sqrt(DBSize)
	targetChunkSize := uint64(2 * math.Sqrt(float64(DBSize)))
	// round up to the next power of 2
	ChunkSize := uint64(1)
	for ChunkSize < targetChunkSize {
		ChunkSize *= 2
	}
	SetSize := uint64(math.Ceil(float64(DBSize) / float64(ChunkSize)))
	// round up to the next mulitple of 4
	SetSize = (SetSize + 3) / 4 * 4
	return ChunkSize, SetSize
}

type AesPrf struct {
	// block cipher.Block
	enc []uint32
}

func xor16(dst, a, b *byte)
func encryptAes128(xk *uint32, dst, src *byte)
func aes128MMO(xk *uint32, dst, src *byte)
func expandKeyAsm(key *byte, enc *uint32)

func NewCipher(key uint64) (*AesPrf, error) {
	k := make([]byte, 16)
	binary.LittleEndian.PutUint64(k, key)
	// n := 11*4
	c := AesPrf{make([]uint32, 4)}
	expandKeyAsm(&k[0], &c.enc[0])
	// fmt.Println("NEW CIPHER")
	// fmt.Println(k)
	// fmt.Println(c.enc)
	return &c, nil
}

func (c *AesPrf) Encrypt(dst, src []byte) {
	encryptAes128(&c.enc[0], &dst[0], &src[0])
}

func DefaultHash(key uint64) uint64 {
	hash := fnv.New64a()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, key)
	hash.Write(b)
	return hash.Sum64()
}

func nonSafePRFEval(key uint64, x uint64) uint64 {
	return DefaultHash(key ^ x)
}

// chacha20 PRF. Slower and not used.
func PRFEval1(key *uint256.Int, x uint64, nonce []byte, y []byte) uint64 {
	aead, _ := chacha20poly1305.New(key.Bytes())
	//nonce := make([]byte, chacha20poly1305.NonceSize)
	//y := make([]byte, 8)
	binary.LittleEndian.PutUint64(y, x)
	ciphertext := aead.Seal(nil, nonce, y, nil)
	return binary.LittleEndian.Uint64(ciphertext)
}

// chacha20 PRF. Slower and not used.
func PRFEval2(key *PrfKey, x uint64) uint64 {
	var nonce PrfNonce
	c, _ := chacha20.New((*key)[:], nonce[:])
	src := make([]byte, 8)
	binary.LittleEndian.PutUint64(make([]byte, 8), x)
	dsc := make([]byte, 8)
	c.XORKeyStream(dsc, src)
	return binary.LittleEndian.Uint64(dsc)
}

// non-safe PRF
func PRFEval3(key *PrfKey128, x uint64) uint64 {
	var key2 PrfKey
	copy(key2[0:16], key[0:16])
	return PRFEval2(&key2, x)
}

// PRF using AES128. Same as Checklist, https://github.com/dimakogan/checklist/tree/master/modules/dpf-go/dpf.
func PRFEval4(key *PrfKey128, x uint64) uint64 {
	var longKey = make([]uint32, 11*4)
	expandKeyAsm(&key[0], &longKey[0])
	var src = make([]byte, 16)
	var dsc = make([]byte, 16)
	binary.LittleEndian.PutUint64(src, x)
	aes128MMO(&longKey[0], &dsc[0], &src[0])
	return binary.LittleEndian.Uint64(dsc)
}

// PRSet is just a wrapper around a PRF key
type PRSet struct {
	Key PrfKey
}

func (p *PRSet) Expand(SetSize uint64, ChunkSize uint64) []uint64 {
	expandedSet := make([]uint64, SetSize)
	for i := uint64(0); i < SetSize; i++ {
		tmp := PRFEval(&p.Key, i)
		offset := tmp & (ChunkSize - 1)
		expandedSet[i] = i*ChunkSize + offset
	}
	return expandedSet
}

func (p *PRSet) MembTest(id uint64, SetSize uint64, ChunkSize uint64) bool {
	ChunkID := id / ChunkSize
	// ensure Chunk size is a power of 2
	ChunkOffset := id & (ChunkSize - 1)

	tmp := PRFEval(&p.Key, ChunkID)
	offset := tmp & (ChunkSize - 1)
	return ChunkOffset == offset
}
