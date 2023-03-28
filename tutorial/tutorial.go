package main

import (
	"log"
	"math"
	"math/rand"
	"time"

	"example.com/util"
)

var DBSize uint64
var ChunkSize uint64
var ChunkNum uint64
var DB []uint64

func PossibleParities(offsetVec []uint64) []uint64 {
	// Run by the server. Given a punctured offset, it first guesses the position of the punctured entry,
	// then it computes the possible parities.
	parities := make([]uint64, ChunkNum)
	parities[0] = 0
	for i := uint64(0); i < ChunkNum-1; i++ {
		xi := (i+1)*ChunkSize + offsetVec[i]
		parities[0] ^= DB[xi]
	}
	for i := uint64(0); i < ChunkNum-1; i++ {
		parities[i+1] = parities[i] ^ DB[(i+1)*ChunkSize+offsetVec[i]] ^ DB[i*ChunkSize+offsetVec[i]]
	}
	return parities
}

type LocalHint struct {
	key             util.PrfKey
	parity          uint64
	programmedPoint uint64
	isProgrammed    bool
}

// Elem returns the element in the chunkID-th chunk of the hint. It takes care of the case when the hint is programmed.
func Elem(hint *LocalHint, chunkId uint64) uint64 {
	if hint.isProgrammed && chunkId == hint.programmedPoint/ChunkSize {
		return hint.programmedPoint
	} else {
		return util.PRFEval(&hint.key, chunkId)%ChunkSize + chunkId*ChunkSize
	}
}

func PIR() {
	// Suppose there's a public DB.
	DB = make([]uint64, DBSize)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < int(DBSize); i++ {
		DB[i] = rng.Uint64()
	}

	// setup the parameters
	ChunkSize = uint64(math.Sqrt(float64(DBSize)))
	ChunkNum = uint64(math.Ceil(float64(DBSize) / float64(ChunkSize)))
	log.Printf("DBSize: %d, ChunkSize: %d, ChunkNum: %d", DBSize, ChunkSize, ChunkNum)

	// The following is the client side algorithm.
	Q := uint64(math.Sqrt(float64(DBSize)) * math.Log(float64(DBSize)))
	M1 := 4 * uint64(math.Sqrt(float64(DBSize))*math.Log(float64(DBSize)))
	M2 := 4 * uint64(math.Log(float64(DBSize)))
	log.Printf("Q: %d, M1: %d, M2: %d", Q, M1, M2)

	//Setup Phase
	//The client first samples the hints
	primaryHints := make([]LocalHint, M1)
	backupHints := make([]LocalHint, M2*ChunkNum)
	for i := uint64(0); i < M1; i++ {
		primaryHints[i] = LocalHint{util.RandKey(rng), 0, 0, false}
	}
	for i := uint64(0); i < M2*ChunkNum; i++ {
		backupHints[i] = LocalHint{util.RandKey(rng), 0, 0, false}
	}
	//The client streamingly downloads the chunks from the server
	for i := uint64(0); i < ChunkNum; i++ {
		// suppose the client receives the i-th chunk, DB[i*ChunkSize:(i+1)*ChunkSize]
		for j := uint64(0); j < M1; j++ {
			primaryHints[j].parity ^= DB[Elem(&primaryHints[j], i)]
		}
		for j := uint64(0); j < M2*ChunkNum; j++ {
			if j/M2 != i {
				backupHints[j].parity ^= DB[Elem(&backupHints[j], i)]
			}
		}
	}

	//Online Query Phase
	localCache := make(map[uint64]uint64)
	consumedHintNum := make([]uint64, ChunkNum)
	for q := uint64(0); q < Q; q++ {
		// just do random query for now
		x := rng.Uint64() % DBSize

		// make sure x is not in the local cache
		for {
			if _, ok := localCache[x]; ok == false {
				break
			}
			x = rng.Uint64() % DBSize
		}

		chunkId := x / ChunkSize
		hitId := uint64(999999999)
		for i := uint64(0); i < M1; i++ {
			if Elem(&primaryHints[i], chunkId) == x {
				hitId = i
				break
			}
		}
		if hitId == uint64(999999999) {
			log.Fatalf("Error: cannot find the hitId")
		}

		offsetVec := make([]uint64, ChunkNum)
		for i := uint64(0); i < ChunkNum; i++ {
			offsetVec[i] = Elem(&primaryHints[hitId], i) % ChunkSize
		}
		punctOffsetVec := offsetVec[0:chunkId]
		punctOffsetVec = append(punctOffsetVec, offsetVec[chunkId+1:]...)

		//send the punctured offset vector to the server and get the parities
		parities := PossibleParities(punctOffsetVec)
		answer := parities[chunkId] ^ primaryHints[hitId].parity

		// This verification only happens in this demo experiment.
		if answer != DB[x] {
			log.Fatalf("Error: answer is not correct")
		}

		// update the local cache
		localCache[x] = answer

		// refresh the hint
		if consumedHintNum[chunkId] < M2 {
			primaryHints[hitId] = backupHints[chunkId*M2+consumedHintNum[chunkId]]
			primaryHints[hitId].isProgrammed = true
			primaryHints[hitId].programmedPoint = x
			primaryHints[hitId].parity ^= answer
			consumedHintNum[chunkId]++
		} else {
			log.Fatalf("Not enough backup hints")
		}
	}
	log.Printf("PIR finished successfully")
}

func main() {
	DBSize = 10000 // please make sure DBSize is a perfect square
	PIR()
}
