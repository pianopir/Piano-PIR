package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	pb "example.com/query"
	util "example.com/util"
	"google.golang.org/grpc"
)

//const (
//	port = ":50051"
//)

var DBSize uint64
var DBSeed uint64
var ChunkSize uint64
var SetSize uint64
var port string

type QueryServiceServer struct {
	pb.UnimplementedQueryServiceServer
	DB []uint64 // the database, for every DBEntrySize/8 uint64s, we store a DBEntry
}

func (s *QueryServiceServer) DBAccess(id uint64) util.DBEntry {
	if id < uint64(len(s.DB)) {
		if id*util.DBEntryLength+util.DBEntryLength > uint64(len(s.DB)) {
			log.Fatalf("DBAccess: id %d out of range", id)
		}
		return util.DBEntryFromSlice(s.DB[id*util.DBEntryLength : (id+1)*util.DBEntryLength])
	} else {
		var ret util.DBEntry
		for i := 0; i < util.DBEntryLength; i++ {
			ret[i] = 0
		}
		return ret
	}
}

// The plaintext query returns the value of the database entry with the given index.
// This is a non-private baseline.
func (s *QueryServiceServer) PlaintextQuery(ctx context.Context, in *pb.PlaintextQueryMsg) (*pb.PlaintextResponse, error) {
	id := in.GetIndex()
	ret := s.DBAccess(id)
	return &pb.PlaintextResponse{Val: ret[:]}, nil
}

// Not needed for the paper.
func (s *QueryServiceServer) HandleFullSetQuery(key util.PrfKey) util.DBEntry {
	PRSet := util.PRSet{Key: key}
	ExpandedSet := PRSet.Expand(SetSize, ChunkSize)

	var parity util.DBEntry
	for i := 0; i < util.DBEntryLength; i++ {
		parity[i] = 0
	}
	for _, id := range ExpandedSet {
		entry := s.DBAccess(id)
		util.DBEntryXor(&parity, &entry)
	}

	return parity
}

// Not needed for the paper.
func (s *QueryServiceServer) FullSetQuery(ctx context.Context, in *pb.FullSetQueryMsg) (*pb.FullSetResponse, error) {
	var key util.PrfKey
	copy(key[:], in.GetPRFKey())
	val := s.HandleFullSetQuery(key)
	return &pb.FullSetResponse{Val: val[:]}, nil
}

// This is the PossibleParities() function in the paper.
func (s *QueryServiceServer) HandlePunctSetQuery(indices []uint64) []uint64 {
	guesses := make([]uint64, SetSize*util.DBEntryLength)
	parity := util.ZeroEntry()

	// build the first guess when the punctured position is 0
	for chunkID, offset := range indices {
		currentID := uint64(chunkID+1)*ChunkSize + offset
		entry := s.DBAccess(currentID)
		util.DBEntryXor(&parity, &entry)
	}

	copy(guesses[0:util.DBEntryLength], parity[:])

	// now build the rest of the guesses
	for i := uint64(1); i < SetSize; i++ {
		//  the hole originally is in the (i-1)-th chunk
		// now the hole should be in the i-th chunk
		offset := indices[i-1]
		oldIndex := uint64(i)*ChunkSize + offset
		newIndex := uint64(i-1)*ChunkSize + offset
		entryOld := s.DBAccess(oldIndex)
		entryNew := s.DBAccess(newIndex)
		util.DBEntryXor(&parity, &entryOld)
		util.DBEntryXor(&parity, &entryNew)
		copy(guesses[i*util.DBEntryLength:(i+1)*util.DBEntryLength], parity[:])
	}

	return guesses
}

// Given a punctured and compacted offset vector, return the corresponding set of guesses.
func (s *QueryServiceServer) PunctSetQuery(ctx context.Context, in *pb.PunctSetQueryMsg) (*pb.PunctSetResponse, error) {
	// start from _, x1, ..., x_{k-1}
	guesses := s.HandlePunctSetQuery(in.GetIndices())
	return &pb.PunctSetResponse{ReturnSize: SetSize, Guesses: guesses}, nil
}

// Not needed for the paper.
func (s *QueryServiceServer) BatchedFullSetQuery(ctx context.Context, in *pb.BatchedFullSetQueryMsg) (*pb.BatchedFullSetResponse, error) {
	num := in.GetQueryNum()
	batchedQuery := in.GetQueries()
	batchedResponse := make([]*pb.FullSetResponse, num)
	for i := uint64(0); i < num; i++ {
		var key util.PrfKey
		copy(key[:], batchedQuery[i].GetPRFKey())
		val := s.HandleFullSetQuery(key)
		batchedResponse[i] = &pb.FullSetResponse{Val: val[:]}
	}

	return &pb.BatchedFullSetResponse{ResponseNum: num, Responses: batchedResponse}, nil
}

// Streamingly send the database to the client. This is used in the preprocessing.
func (s *QueryServiceServer) FetchFullDB(in *pb.FetchFullDBMsg, stream pb.QueryService_FetchFullDBServer) error {
	for i := uint64(0); i < SetSize; i++ {
		down := i * ChunkSize
		up := (i + 1) * ChunkSize
		var chunk []uint64
		chunk = s.DB[down*util.DBEntryLength : up*util.DBEntryLength]

		ret := &pb.DBChunk{ChunkId: i, ChunkSize: ChunkSize, Chunk: chunk}
		if err := stream.Send(ret); err != nil {
			log.Printf("Failed to send a chunk: %v", err)
			return err
		}
	}
	return nil
}

// Read the database size and the seed from the config file.
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

	if DBSize, err = strconv.ParseUint(split[0], 10, 32); err != nil {
		log.Fatal(err)
	}
	if DBSeed, err = strconv.ParseUint(split[1], 10, 32); err != nil {
		log.Fatal(err)
	}

	log.Printf("%v %v", DBSize, DBSeed)

	return uint64(DBSize), uint64(DBSeed)
}

func main() {
	portPtr := flag.String("port", "50051", "port number")
	flag.Parse()

	port = *portPtr
	log.Println("port number: ", port)
	port = ":" + port

	DBSize, DBSeed = ReadConfigInfo()
	log.Printf("DB N: %v, Entry Size %v Bytes, DB Size %v MB", DBSize, util.DBEntrySize, DBSize*util.DBEntrySize/1024/1024)

	ChunkSize, SetSize = util.GenParams(DBSize)

	log.Println("Chunk Size:", ChunkSize, "Set Size:", SetSize)

	// listen on TCP port
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen %v", err)
	}

	// set up the database
	// pad the db to ChunkSize*SetSize
	DB := make([]uint64, ChunkSize*SetSize*util.DBEntryLength)
	log.Println("DB Real N:", len(DB))
	for i := uint64(0); i < DBSize; i++ {
		entry := util.GenDBEntry(DBSeed, i)
		copy(DB[i*util.DBEntryLength:(i+1)*util.DBEntryLength], entry[:])
	}
	// the padding part is all 0
	for i := DBSize * util.DBEntryLength; i < uint64(len(DB)); i++ {
		DB[i] = 0
	}

	// set the max message size to 12MB
	maxMsgSize := 12 * 1024 * 1024

	// create a gRPC server object
	s := grpc.NewServer(
		grpc.MaxMsgSize(maxMsgSize),
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize),
	)

	pb.RegisterQueryServiceServer(s, &QueryServiceServer{DB: DB[:]})
	log.Printf("server listening at %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to server %v", err)
	}
}
