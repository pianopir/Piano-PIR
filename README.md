### Warning:  This is only a temporary repo. To see the updated implementation, see https://github.com/wuwuz/Piano-PIR-new (Coming out soon).


# Piano: Extremely Simple, Single-server PIR with Sublinear Server Computation

This is a prototype implementation of the Piano private information retrieval(PIR) algorithm that allows a client to access a database without the server knowing the querying index.
The algorithm details can be found in the paper(coming soon...).

**Warning**: The code is not audited and not for any serious commercial or real-world use case. Please use it only for educational purpose.

### Prerequisite:
1. Install Go(https://go.dev/doc/install).
2. For developing, please install gRPC(https://grpc.io/docs/languages/go/quickstart/)

### A Mini Tutorial

The tutorial implementation is in `tutorial/tutorial.go`.

Try `go run tutorial/tutorial.go`.

### Running Experiments:
1. In one terminal, `go run server/server.go -port 50051`. This sets up the server. The server will store the whole DB in the RAM, so please ensure there's enough memory.
2. In another terminal, `go run client/client.go -ip localhost:50051 -thread 8`. This runs the PIR experiment with one setup phase for a window of $\sqrt{n}\ln(n)$-queries and follows with the online phase of up to 1000 queries. The ip flag denotes the server's adddress. The thread denotes how many threads are used in the setup phase. Usually 4 and 8 threads provide around 3x and 6x improvement.

#### Different DB configuration:
1. The two integers in `config.txt` denote `N` and `DBSeed`. `N` denotes the number of entries in the database. `DBSeed` denotes the random seed to generate the DB. The client will use the seed only for verifying the correctness. The code only reads the integers in the first line.
2. In `util/util.go`, you can change the `DBEntrySize` constant to change the entry size, e.g. 8bytes, 32bytes, 256bytes.

The default is `N=33554432` and `DBEntrySize=8`, which is a 256MB DB.
For another example, setting `N=134217728`  and `DBEntrySize=16` will generate a 2GB database.

### Developing
1. The server implementation is in `server/server.go`.
2. The client implementation is in `client/client.go`.
3. Common utilities are in `util/util.go`, including the PRF and the `DBEntry` definition.
4. The messages exchanged by servers and client are defined in `query/query.proto`. If you change it, run `bash proto.sh` to generate the corresponding server and client API. You should implement those API later.

### Contact

Removed for anonymity
