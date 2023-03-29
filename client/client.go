package main

import (
	"bufio"
	"context"
	"flag"
	"reflect"
	"sync"

	"fmt"
	//"encoding/binary"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	pb "example.com/query"
	"example.com/util"
	"google.golang.org/grpc"
)

const (
	leftAddress     = "localhost:50051"
	rightAddress    = "localhost:50052"
	FailureProbLog2 = 40
)

var DBSize uint64
var DBSeed uint64
var ChunkSize uint64
var SetSize uint64
var threadNum uint64
var serverAddr string
var LogFile *os.File
var str string

// PlainTextQuery, only for testing network
func runSingleQuery(client pb.QueryServiceClient, DBSize uint64, DBSeed uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Millisecond*500000000000))
	defer cancel()

	log.Printf("Running Network Test")

	rng := rand.New(rand.NewSource(103))
	Q := 1000

	start := time.Now()
	for i := 0; i < Q; i++ {
		j := rng.Uint64() % DBSize
		res, err := client.PlaintextQuery(ctx, &pb.PlaintextQueryMsg{Index: j})

		if len(res.Val) != util.DBEntryLength {
			log.Fatalf("the return value length is %v. Querying for %v", len(res.Val), j)
		}

		resEntry := util.DBEntryFromSlice(res.Val)

		if err != nil {
			log.Fatalf("failed to query %v", err)
		}

		correctVal := util.GenDBEntry(j, DBSeed)

		if util.EntryIsEqual(&resEntry, &correctVal) == false {
			log.Fatalf("wrong value %v at index %v", res.Val, j)
		}
	}
	elapsed := time.Since(start)
	log.Printf("Non-Private Time: %v ms", float64(elapsed.Milliseconds())/float64(Q))
}

// Query a set of indices. Not used by in the paper.
func runFullSetQuery(client pb.QueryServiceClient, DBSize uint64, DBSeed uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Millisecond*500))
	defer cancel()

	rng := rand.New(rand.NewSource(103))
	for i := 0; i < 100; i++ {
		key := util.RandKey(rng)
		res, err := client.FullSetQuery(ctx, &pb.FullSetQueryMsg{PRFKey: key[:]})
		resEntry := util.DBEntryFromSlice(res.Val)

		parity := util.ZeroEntry()
		set := util.PRSet{Key: key}
		ExpandedSet := set.Expand(SetSize, ChunkSize)
		for _, id := range ExpandedSet {
			//log.Printf("%v ", id)
			if id < DBSize {
				entry := util.GenDBEntry(id, DBSeed)
				util.DBEntryXor(&parity, &entry)
			}
		}

		if err != nil {
			log.Fatalf("failed to query %v", err)
		}

		if util.EntryIsEqual(&resEntry, &parity) == false {
			log.Fatalf("wrong value %v at key %v", res.Val, key)
		}

		if i%10 == 0 {
			log.Printf("Got %v-th answer: ", i)
		}
	}
}

// Query a set of indices. Not used by in the paper.
func runBatchedFullSetQuery(client pb.QueryServiceClient, DBSize uint64, DBSeed uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Millisecond*500))
	defer cancel()

	rng := rand.New(rand.NewSource(103))
	batchedFullSetQuery := make([]*pb.FullSetQueryMsg, 0)
	queryNum := uint64(100)
	for i := uint64(0); i < queryNum; i++ {
		//bucket, err := client.SingleQuery(ctx, &pb.CuckooBucketQuery{QueryNum: 1, BucketId: [uint64(i % 10)]})
		key := util.RandKey(rng)
		batchedFullSetQuery = append(batchedFullSetQuery, &pb.FullSetQueryMsg{PRFKey: key[:]})
	}

	res, err := client.BatchedFullSetQuery(ctx, &pb.BatchedFullSetQueryMsg{QueryNum: queryNum, Queries: batchedFullSetQuery})

	if err != nil {
		log.Fatalf("failed to query %v", err)
	}

	for i := uint64(0); i < queryNum; i++ {
		key := batchedFullSetQuery[i].PRFKey
		var prfKey util.PrfKey
		copy(prfKey[:], key)
		parity := util.ZeroEntry()
		set := util.PRSet{Key: prfKey}
		ExpandedSet := set.Expand(SetSize, ChunkSize)
		for _, id := range ExpandedSet {
			//log.Printf("%v ", id)
			if id < DBSize {
				entry := util.GenDBEntry(id, DBSeed)
				util.DBEntryXor(&parity, &entry)
			}
		}

		resEntry := util.DBEntryFromSlice(res.Responses[i].Val)
		if util.EntryIsEqual(&resEntry, &parity) == false {
			log.Fatalf("wrong value %v at key %v", parity, key)
		}

		if i%10 == 0 {
			log.Printf("Got %v-th answer: ", i)
		}
	}
}

// Query a punctured set. Only for testing. Not used by in the paper.
func runPunctSetQuery(client pb.QueryServiceClient, DBSize uint64, DBSeed uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Millisecond*500))
	defer cancel()

	rng := rand.New(rand.NewSource(103))
	for i := 0; i < 100; i++ {
		//bucket, err := client.SingleQuery(ctx, &pb.CuckooBucketQuery{QueryNum: 1, BucketId: [uint64(i % 10)]})
		key := util.RandKey(rng)
		set := util.PRSet{Key: key}
		expandSet := set.Expand(SetSize, ChunkSize)

		punctChunkId := rng.Intn(len(expandSet))
		punctSet := make([]uint64, len(expandSet)-1)
		for i := uint64(0); i < SetSize; i++ {
			if i < uint64(punctChunkId) {
				punctSet[i] = expandSet[i] & (ChunkSize - 1)
			}
			if i > uint64(punctChunkId) {
				punctSet[i-1] = expandSet[i] & (ChunkSize - 1)
			}
		}
		res, err := client.PunctSetQuery(ctx, &pb.PunctSetQueryMsg{PunctSetSize: uint64(len(punctSet)), Indices: punctSet})

		parity := util.ZeroEntry()
		for chunkId, id := range expandSet {
			if i == 0 {
				log.Printf("chunkId %v id %v", chunkId, id)
			}
			if chunkId != punctChunkId {
				if id < DBSize {
					entry := util.GenDBEntry(id, DBSeed)
					util.DBEntryXor(&parity, &entry)
				}
			} else {
				if i == 0 {
					log.Println("punct", chunkId)
				}
			}
		}

		if err != nil {
			log.Fatalf("failed to query %v", err)
		}

		log.Println("parity", parity)

		resEntry := util.DBEntryFromSlice(res.Guesses[punctChunkId*util.DBEntryLength : (punctChunkId+1)*util.DBEntryLength])
		if util.EntryIsEqual(&resEntry, &parity) == false {
			log.Fatalf("wrong value, parity = %v, res = %v, at key %v", parity, resEntry, key)
		}

		if i%10 == 0 {
			log.Printf("Got %v-th answer: ", i)
		}
	}
}

// read config file, return DBSize and DBSeed
func ReadConfigInfo() (uint64, uint64) {
	file, err := os.Open("config.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	line, _, err := reader.ReadLine()
	if err != nil {
		log.Fatal(err)
	}
	split := strings.Split(string(line), " ")
	var DBSize uint64
	var DBSeed uint64

	if DBSize, err = strconv.ParseUint(split[0], 10, 64); err != nil {
		log.Fatal(err)
	}
	if DBSeed, err = strconv.ParseUint(split[1], 10, 64); err != nil {
		log.Fatal(err)
	}

	log.Printf("%v %v", DBSize, DBSeed)

	return uint64(DBSize), uint64(DBSeed)
}

// Primary Hint Sturctures
type LocalSet struct {
	key             util.PrfKey
	parity          util.DBEntry
	programmedPoint uint64
	isProgrammed    bool
}

// Backup Hint Structures
type LocalBackupSet struct {
	key              util.PrfKey
	parityAfterPunct util.DBEntry
}

// Backup Hint Groups
type LocalBackupSetGroup struct {
	consumed uint64
	sets     []LocalBackupSet
}

// Given a target failure probability, return the number of primary hints
// For any query and any hint, the hint contains the query with prob 1/ChunkSize
// So if we have k*ChunkSize hints, the failure probability is less than (1/ChunkSize)^(k*ChunkSize) <= (1/e)^k
// We have Q queries
// So we need Q * e^(-k) <= 2^(-target)
// k = ln(2)*(target) + ln(Q)
func primaryNumParam(Q float64, ChunkSize float64, target float64) uint64 {
	k := math.Ceil(math.Log(2)*(target) + math.Log(Q))
	//log.Printf("k = %v", k)
	return uint64(k) * uint64(ChunkSize)
}

// Return a loose bound of balls into bins failure probability with Chernoff bound
// This is too loose and not used. We mauanlly calculate the tail probability of the binomial distribution
// And verify the parameters are ok for 2^(-41).
func FailProbBallIntoBins(ballNum uint64, binNum uint64, binSize uint64) float64 {
	//log.Printf("ballNum = %v, binNum = %v, binSize = %v", ballNum, binNum, binSize)
	mean := float64(ballNum) / float64(binNum)
	c := float64(binSize)/mean - 1
	log.Printf("mean = %v, c = %v", mean, c)
	// chernoff exp(-(c^2)/(2+c) * mean)
	t := (mean * (c * c) / (2 + c)) * math.Log(2)
	t -= math.Log2(float64(binNum))
	//log.Printf("t = %v", t)
	return t
}

// The main client function
func runPIRWithOneServer(leftClient pb.QueryServiceClient, DBSize uint64, DBSeed uint64) {
	// Set up a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Millisecond*100000000000))
	defer cancel()

	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))

	// Setup the parameters
	// We just query for Q=nln(n) queries
	totalQueryNum := uint64(math.Sqrt(float64(DBSize)) * math.Log(float64(DBSize)))
	plannedQueryNum := totalQueryNum
	log.Printf("totalQueryNum %v", totalQueryNum)

	// This is M1 in the paper. We make it big enough so that the failure probability is 2^{-FailureProbLog2-1}
	localSetNum := primaryNumParam(float64(totalQueryNum), float64(ChunkSize), FailureProbLog2+1)
	// Since we have multiple threads, we need to make sure the number of local sets is a multiple of the number of threads
	localSetNum = (localSetNum + threadNum - 1) / threadNum * threadNum

	// This is the M2 in the paper.
	backupSetNumPerGroup := 3 * uint64(float64(totalQueryNum)/float64(SetSize))
	// Since we have multiple threads, we need to make sure the number of backup sets is a multiple of the number of threads
	backupSetNumPerGroup = (backupSetNumPerGroup + threadNum - 1) / threadNum * threadNum
	totalBackupSetNum := backupSetNumPerGroup * SetSize

	// For the experiment, we only run 1000 queries. The plannedQueryNum is the number of queries that we can support
	if totalQueryNum > 1000 {
		totalQueryNum = 1000
	}

	// Setup Phase:
	// 		The client sends a simple query to the server to fetch the whole DB
	//		When the client gets i-th chunk, it updates all local sets' parities
	//		It also needs to update the backup sets' parities
	//		It also needs to update the i-th group's punct point parity
	start := time.Now()

	// Initialize local sets and backup sets

	localSets := make([]LocalSet, localSetNum)
	localBackupSets := make([]LocalBackupSet, totalBackupSetNum)
	localCache := make(map[uint64]util.DBEntry)
	localMissElements := make(map[uint64]util.DBEntry)

	for j := uint64(0); j < localSetNum; j++ {
		localSets[j] = LocalSet{
			key:             util.RandKey(rng),
			parity:          util.ZeroEntry(),
			isProgrammed:    false,
			programmedPoint: 0,
		}
	}

	LocalBackupSetGroups := make([]LocalBackupSetGroup, SetSize)

	for i := uint64(0); i < SetSize; i++ {
		LocalBackupSetGroups[i].consumed = 0
		LocalBackupSetGroups[i].sets = localBackupSets[i*backupSetNumPerGroup : (i+1)*backupSetNumPerGroup]
	}

	for j := uint64(0); j < SetSize; j++ {
		for k := uint64(0); k < backupSetNumPerGroup; k++ {
			LocalBackupSetGroups[j].sets[k] = LocalBackupSet{
				key:              util.RandKey(rng),
				parityAfterPunct: util.ZeroEntry(),
			}
		}
	}

	// now fetch the whole DB
	log.Printf("Start fetching the whole DB")
	// print the size of LocalSet using reflection
	log.Printf("Every Local Set Size %v bytes", reflect.TypeOf(LocalSet{}).Size())
	log.Printf("Every Local Backup Set Size %v bytes", reflect.TypeOf(LocalBackupSet{}).Size())
	log.Printf("Local Set Num %v, Local Backup Set Num %v", localSetNum, totalBackupSetNum)
	log.Printf("Local Storage Size %v MB", float64(localSetNum*uint64(reflect.TypeOf(LocalSet{}).Size())+(totalBackupSetNum*uint64(reflect.TypeOf(LocalBackupSet{}).Size())))/1024/1024)
	log.Printf("Per query communication cost %v kb", float64(SetSize)*float64(8+uint64(reflect.TypeOf(util.DBEntry{}).Size()))/1024)

	str = fmt.Sprintf("Every Local Set Size %v bytes\n", reflect.TypeOf(LocalSet{}).Size())
	LogFile.WriteString(str)

	str = fmt.Sprintf("Every Local Backup Set Size %v bytes\n", reflect.TypeOf(LocalBackupSet{}).Size())
	LogFile.WriteString(str)

	str = fmt.Sprintf("Local Set Num %v, Local Backup Set Num %v\n", localSetNum, totalBackupSetNum)
	LogFile.WriteString(str)

	str = fmt.Sprintf("Local Storage Size %v MB\n", float64(localSetNum*uint64(reflect.TypeOf(LocalSet{}).Size())+(totalBackupSetNum*uint64(reflect.TypeOf(LocalBackupSet{}).Size())))/1024/1024)
	LogFile.WriteString(str)

	str = fmt.Sprintf("Per query communication cost %v kb\n", float64(SetSize)*float64(8+uint64(reflect.TypeOf(util.DBEntry{}).Size()))/1024)
	LogFile.WriteString(str)

	fechFullDBMsg := &pb.FetchFullDBMsg{Dummy: 1}
	stream, err := leftClient.FetchFullDB(ctx, fechFullDBMsg)

	if err != nil {
		log.Fatalf("failed to fetch full DB %v", err)
	}

	for i := uint64(0); i < SetSize; i++ {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("failed to receive chunk %v", err)
		}
		if i%1000 == 0 {
			log.Printf("received chunk %v", i)
		}

		//hitMap is irrelavant to the paper. We want to know if any indices are missed.
		hitMap := make([]bool, ChunkSize)

		// use threadNum threads to parallelize the computation for the chunk
		var wg sync.WaitGroup
		wg.Add(int(threadNum))

		perTheadSetNum := (localSetNum+threadNum-1)/threadNum + 1 // make sure all sets are covered
		perThreadBackupNum := (totalBackupSetNum+threadNum-1)/threadNum + 1

		for tid := uint64(0); tid < threadNum; tid++ {
			startIndex := uint64(tid) * uint64(perTheadSetNum)
			endIndex := startIndex + uint64(perTheadSetNum)
			if endIndex > localSetNum {
				endIndex = localSetNum
			}

			startIndexBackup := uint64(tid) * uint64(perThreadBackupNum)
			endIndexBackup := startIndexBackup + uint64(perThreadBackupNum)
			if endIndexBackup > totalBackupSetNum {
				endIndexBackup = totalBackupSetNum
			}

			go func(start, end, start1, end1 uint64) {
				defer wg.Done()
				// update the parity for the primary hints
				for j := uint64(start); j < uint64(end); j++ {
					tmp := util.PRFEval(&localSets[j].key, i)
					offset := tmp & (ChunkSize - 1)
					hitMap[offset] = true
					util.DBEntryXorFromRaw(&localSets[j].parity, chunk.Chunk[offset*util.DBEntryLength:(offset+1)*util.DBEntryLength])
				}
				//update the parity for the backup hints
				for j := uint64(start1); j < uint64(end1); j++ {
					tmp := util.PRFEval(&localBackupSets[j].key, i)
					offset := tmp & (ChunkSize - 1)
					util.DBEntryXorFromRaw(&localBackupSets[j].parityAfterPunct, chunk.Chunk[offset*util.DBEntryLength:(offset+1)*util.DBEntryLength])
				}
			}(startIndex, endIndex, startIndexBackup, endIndexBackup)
		}

		wg.Wait()

		// if some indices are missed, we need to fetch the corresponding elements
		for j := uint64(0); j < ChunkSize; j++ {
			if hitMap[j] == false {
				entry := util.DBEntryFromSlice(chunk.Chunk[j*util.DBEntryLength : (j+1)*util.DBEntryLength])
				localMissElements[j+i*ChunkSize] = entry
			}
		}

		// puncture at i-th chunk for the i-th group. Just xor them again.
		for k := uint64(0); k < backupSetNumPerGroup; k++ {
			key := &LocalBackupSetGroups[i].sets[k].key
			tmp := util.PRFEval(key, i)
			offset := tmp & (ChunkSize - 1)
			util.DBEntryXorFromRaw(&LocalBackupSetGroups[i].sets[k].parityAfterPunct, chunk.Chunk[offset*util.DBEntryLength:(offset+1)*util.DBEntryLength])
		}
	}

	elapsed := time.Since(start)
	offlineElapsed := elapsed
	offlineCommCost := float64(DBSize) * float64(reflect.TypeOf(util.DBEntry{}).Size())

	log.Printf("Finish Setup Phase, store %v local sets, %v backup sets", localSetNum, SetSize*backupSetNumPerGroup)
	log.Printf("Local Storage Size %v MB", float64(localSetNum*uint64(reflect.TypeOf(LocalSet{}).Size())+(totalBackupSetNum*uint64(reflect.TypeOf(LocalBackupSet{}).Size())))/1024/1024)
	log.Printf("Setup Phase took %v ms, amortized time %v ms per query", elapsed.Milliseconds(), float64(elapsed.Milliseconds())/float64(plannedQueryNum))
	log.Printf("Setup Phase Comm Cost %v MB, amortized cost %v KB per query", float64(offlineCommCost)/1024/1024, float64(offlineCommCost)/1024/float64(plannedQueryNum))
	log.Printf("Num of local miss elements %v", len(localMissElements))
	str = fmt.Sprintf("Finish Setup Phase, store %v local sets, %v backup sets\n", localSetNum, SetSize*backupSetNumPerGroup)
	LogFile.WriteString(str)

	str = fmt.Sprintf("Local Storage Size %v MB\n", float64(localSetNum*uint64(reflect.TypeOf(LocalSet{}).Size())+(totalBackupSetNum*uint64(reflect.TypeOf(LocalBackupSet{}).Size())))/1024/1024)
	LogFile.WriteString(str)

	str = fmt.Sprintf("Setup Phase took %v ms, amortized time %v ms per query\n", elapsed.Milliseconds(), float64(elapsed.Milliseconds())/float64(totalQueryNum))
	LogFile.WriteString(str)

	str = fmt.Sprintf("Setup Phase Comm Cost %v MB, amortized cost %v KB per query", float64(offlineCommCost)/1024/1024, float64(offlineCommCost)/1024/float64(plannedQueryNum))
	LogFile.WriteString(str)

	str = fmt.Sprintf("Num of local miss elements %v\n", len(localMissElements))
	LogFile.WriteString(str)

	// Online Query Phase:
	start = time.Now()

	for q := uint64(0); q < totalQueryNum; q++ {
		if q%10000 == 0 {
			log.Printf("Making %v-th query", q)
		}
		// just do random query for now
		x := rng.Uint64() % DBSize

		// make sure x is not in the local cache
		for true {
			if _, ok := localCache[x]; ok == false {
				break
			}
			x = rng.Uint64() % DBSize
		}

		// 		1. Query x: the client first finds a local set that contains x
		// 		2. The client expands the set and punctures the set at x
		// 		3. The client sends the punctured set to the right server and gets sqrt(n)-1 guesses
		//      4. The client picks the correct guesses and xors with the local parity to get DB[x]

		hitSetId := uint64(999999999)

		// search for the local hint that contains x
		for i := uint64(0); i < localSetNum; i++ {
			tmpKey := localSets[i].key
			set := util.PRSet{Key: tmpKey}
			if localSets[i].isProgrammed && (x/ChunkSize) == (localSets[i].programmedPoint/ChunkSize) {
				if x == localSets[i].programmedPoint {
					hitSetId = i
					break
				}
			} else {
				if set.MembTest(x, SetSize, ChunkSize) {
					hitSetId = i
					break
				}
			}
		}

		var xVal util.DBEntry

		// if still no hit set found, then fail
		if hitSetId == 999999999 {
			if v, ok := localMissElements[x]; ok == false {
				log.Fatalf("No hit set found for %v in %v-th query", x, q)
			} else {
				// if the element is in the local miss elements' cache, then we can just return the value
				log.Printf("Hit missing and cached element %v", x)
				xVal = v
				punctSet := make([]uint64, SetSize-1)
				for i := uint64(0); i < SetSize-1; i++ {
					punctSet[i] = rng.Uint64() % ChunkSize
				}
				// send the dummy punctured set to the server
				_, err := leftClient.PunctSetQuery(ctx, &pb.PunctSetQueryMsg{PunctSetSize: uint64(len(punctSet)), Indices: punctSet})
				if err != nil {
					log.Fatalf("failed to make punct set query to server %v", err)
				}
				localCache[x] = xVal
				continue
			}
		}

		// expand the set
		set := util.PRSet{Key: localSets[hitSetId].key}
		expandedSet := set.Expand(SetSize, ChunkSize)
		// manually program the set if the flag is set
		if localSets[hitSetId].isProgrammed {
			chunkId := localSets[hitSetId].programmedPoint / ChunkSize
			expandedSet[chunkId] = localSets[hitSetId].programmedPoint
		}

		// puncture the set by removing x from the offset vector
		punctChunkId := x / ChunkSize
		punctSet := make([]uint64, SetSize-1)
		for i := uint64(0); i < SetSize; i++ {
			if i < punctChunkId {
				punctSet[i] = expandedSet[i] & (ChunkSize - 1)
			}
			if i > punctChunkId {
				punctSet[i-1] = expandedSet[i] & (ChunkSize - 1)
			}
		}

		// send the punctured set to the server
		res, err := leftClient.PunctSetQuery(ctx, &pb.PunctSetQueryMsg{PunctSetSize: uint64(len(punctSet)), Indices: punctSet})
		if err != nil {
			log.Fatalf("failed to make punct set query to server %v", err)
		}

		// pick the correct guesses, which should be the punctChunkId-th guess
		xVal = localSets[hitSetId].parity
		util.DBEntryXorFromRaw(&xVal, res.Guesses[punctChunkId*util.DBEntryLength:(punctChunkId+1)*util.DBEntryLength])

		// update the local cache
		localCache[x] = xVal

		// verify the correctness of the query. This is for debugging purpose.
		entry := util.GenDBEntry(DBSeed, x)
		if util.EntryIsEqual(&xVal, &entry) == false {
			log.Fatalf("wrong value %v at index %v at query %v", xVal, x, q)
		} else {
			//log.Printf("Correct value %v at index %v", xVal, x)
		}

		// Regresh Phase:
		// The client picks one set from the punctChunkId-th group
		// adds the x to the set and adds the set to the local set list

		if LocalBackupSetGroups[punctChunkId].consumed == backupSetNumPerGroup {
			// if no backup set is available, then fail
			log.Printf("consumed %v sets", LocalBackupSetGroups[punctChunkId].consumed)
			log.Printf("backupSetNumPerGroup %v", backupSetNumPerGroup)
			log.Fatalf("No backup set available for %v-th query", q)
		}

		consumed := LocalBackupSetGroups[punctChunkId].consumed
		localSets[hitSetId].key = LocalBackupSetGroups[punctChunkId].sets[consumed].key
		util.DBEntryXor(&xVal, &LocalBackupSetGroups[punctChunkId].sets[consumed].parityAfterPunct)
		localSets[hitSetId].parity = xVal
		localSets[hitSetId].isProgrammed = true
		localSets[hitSetId].programmedPoint = x
		LocalBackupSetGroups[punctChunkId].consumed++
	}

	elapsed = time.Since(start)
	perQueryUploadCost := float64(SetSize) * float64(8)
	perQueryDownloadCost := float64(SetSize) * float64(uint64(reflect.TypeOf(util.DBEntry{}).Size()))

	log.Printf("Finish Online Phase with %v queries", totalQueryNum)
	log.Printf("Online Phase took %v ms, amortized time %v ms", elapsed.Milliseconds(), float64(elapsed.Milliseconds())/float64(totalQueryNum))
	log.Printf("Per query upload cost %v kb", perQueryUploadCost/1024)
	log.Printf("Per query download cost %v kb", perQueryDownloadCost/1024)
	log.Printf("End to end amortized time %v ms", float64(offlineElapsed.Milliseconds())/float64(plannedQueryNum)+float64(elapsed.Milliseconds())/float64(totalQueryNum))
	log.Printf("End to end amortized comm cost %v kb", (float64(offlineCommCost)/1024/float64(plannedQueryNum) + (perQueryUploadCost+perQueryDownloadCost)/1024))

	str = fmt.Sprintf("Finish Online Phase with %v queries\n", totalQueryNum)
	LogFile.WriteString(str)

	str = fmt.Sprintf("Online Phase took %v ms, amortized time %v ms\n", elapsed.Milliseconds(), float64(elapsed.Milliseconds())/float64(totalQueryNum))
	LogFile.WriteString(str)

	str = fmt.Sprintf("Per query upload cost %v kb\n", float64(perQueryUploadCost)/1024)
	LogFile.WriteString(str)

	str = fmt.Sprintf("Per query download cost %v kb\n", float64(perQueryDownloadCost)/1024)
	LogFile.WriteString(str)

	str = fmt.Sprintf("End to end amortized time %v ms", float64(offlineElapsed.Milliseconds())/float64(plannedQueryNum)+float64(elapsed.Milliseconds())/float64(totalQueryNum))
	LogFile.WriteString(str)

	str = fmt.Sprintf("End to end amortized comm cost %v kb", (float64(offlineCommCost)/1024/float64(plannedQueryNum) + (perQueryUploadCost+perQueryDownloadCost)/1024))
	LogFile.WriteString(str)

}

func main() {
	addrPtr := flag.String("ip", "localhost:50051", "port number")
	threadPtr := flag.Int("thread", 1, "number of threads")
	flag.Parse()

	serverAddr = *addrPtr
	threadNum = uint64(*threadPtr)
	log.Printf("Server address %v, thread number %v", serverAddr, threadNum)

	DBSize, DBSeed = ReadConfigInfo()
	ChunkSize, SetSize = util.GenParams(DBSize)

	// set the max message size of gRPC to 12MB
	maxMsgSize := 12 * 1024 * 1024

	f, _ := os.OpenFile("output.txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	LogFile = f

	log.Printf("DBSize %v, DBSeed %v, ChunkSize %v, SetSize %v", DBSize, DBSeed, ChunkSize, SetSize)

	// connect to the server
	leftConn, err := grpc.Dial(
		serverAddr,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize), grpc.MaxCallSendMsgSize(maxMsgSize)),
	)
	if err != nil {
		log.Fatalf("Failed to connect server %v", leftAddress)
	}
	leftClient := pb.NewQueryServiceClient(leftConn)

	defer leftConn.Close()

	// we don't need the right server now because our scheme is single-server!
	/*
		rightConn, err := grpc.Dial(rightAddress, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Failed to connect server %v", rightAddress)
		}
		defer rightConn.Close()

		rightClient := pb.NewQueryServiceClient(rightConn)
	*/

	// run the plaintext query. This is for network testing.
	runSingleQuery(leftClient, DBSize, DBSeed)
	runPIRWithOneServer(leftClient, DBSize, DBSeed)
}
