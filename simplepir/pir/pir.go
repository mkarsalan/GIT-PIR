package pir

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"time"
	"github.com/mohae/deepcopy"
	// "math"
	"strconv"
	"sync"
	// "reflect"
	// "strings"
)

// Defines the interface for PIR with preprocessing schemes
type PIR interface {
	Name() string

	PickParams(N, d, n, logq uint64) Params
	PickParamsGivenDimensions(l, m, n, logq uint64) Params

	GetBW(info DBinfo, p Params)

	Init(info DBinfo, p Params) State
	InitCompressed(info DBinfo, p Params) (State, CompressedState)
	DecompressState(info DBinfo, p Params, comp CompressedState) State

	Setup(DB *Database, shared State, p Params) (State, Msg)
	FakeSetup(DB *Database, p Params) (State, float64) // used for benchmarking online phase

	Query(i uint64, shared State, p Params, info DBinfo) (State, Msg)

	Answer(DB *Database, query MsgSlice, server State, shared State, p Params) Msg

	Recover(i uint64, batch_index uint64, offline Msg, query Msg, answer Msg, shared State, client State,
		p Params, info DBinfo) uint64

	RecoverFile(i uint64, batch_index uint64, offline Msg, query Msg, answer Msg, shared State, client State,
		p Params, info DBinfo, metadata []uint64) uint64

	RecoverRepository(i uint64, batch_index uint64, offline Msg, query Msg, answer Msg, shared State, client State,
		p Params, info DBinfo, metadata []uint64) []uint64

	Reset(DB *Database, p Params) // reset DB to its correct state, if modified during execution
}


type AnswerChannel struct {
    Index  	int
    Answer 	Msg
}

type QueryChannel struct {
    Index 	int
    Query 	MsgSlice
    States 	[]State
}

type ReconstructingAnswerChannel struct {
    Index  	int
    Answer 	[]uint64
}


// Run full PIR scheme (offline + online phases), where the transmission of the A matrix is compressed.
func RunPIRCompressed(pi PIR, DB *Database, p Params, i []uint64) (float64, float64) {
        fmt.Printf("Executing %s\n", pi.Name())
        //fmt.Printf("Memory limit: %d\n", debug.SetMemoryLimit(math.MaxInt64))
        debug.SetGCPercent(-1)

        num_queries := uint64(len(i))
        if DB.Data.Rows/num_queries < DB.Info.Ne {
                panic("Too many queries to handle!")
        }
        batch_sz := DB.Data.Rows / (DB.Info.Ne * num_queries) * DB.Data.Cols
        bw := float64(0)

        server_shared_state, comp_state := pi.InitCompressed(DB.Info, p)
        client_shared_state := pi.DecompressState(DB.Info, p, comp_state)

        fmt.Println("Setup...")
        start := time.Now()
        server_state, offline_download := pi.Setup(DB, server_shared_state, p)
        printTime(start)
        comm := float64(offline_download.Size() * uint64(p.Logq) / (8.0 * 1024.0))
        fmt.Printf("\t\tOffline download: %f KB\n", comm)
        bw += comm
        runtime.GC()

        fmt.Println("Building query...")
        start = time.Now()
        var client_state []State
        var query MsgSlice
        for index, _ := range i {
                index_to_query := i[index] + uint64(index)*batch_sz
                cs, q := pi.Query(index_to_query, client_shared_state, p, DB.Info)
                client_state = append(client_state, cs)
                query.Data = append(query.Data, q)
        }
        runtime.GC()
        printTime(start)
        comm = float64(query.Size() * uint64(p.Logq) / (8.0 * 1024.0))
        fmt.Printf("\t\tOnline upload: %f KB\n", comm)
        bw += comm
        runtime.GC()

        fmt.Println("Answering query...")
        start = time.Now()
        answer := pi.Answer(DB, query, server_state, server_shared_state, p)
        // elapsed := printTime(start)
        // rate := printRate(p, elapsed, len(i))
        comm = float64(answer.Size() * uint64(p.Logq) / (8.0 * 1024.0))
        fmt.Printf("\t\tOnline download: %f KB\n", comm)
        bw += comm
        runtime.GC()

        pi.Reset(DB, p)
        fmt.Println("Reconstructing...")
        start = time.Now()

        for index, _ := range i {
                index_to_query := i[index] + uint64(index)*batch_sz
                val := pi.Recover(index_to_query, uint64(index), offline_download,
                                  query.Data[index], answer, client_shared_state,
                                  client_state[index], p, DB.Info)

                if DB.GetElem(index_to_query) != val {
                        fmt.Printf("Batch %d (querying index %d -- row should be >= %d): Got %d instead of %d\n",
                                index, index_to_query, DB.Data.Rows/4, val, DB.GetElem(index_to_query))
                        panic("Reconstruct failed!")
                }
        }
        fmt.Println("Success!")
        printTime(start)

        runtime.GC()
        debug.SetGCPercent(100)
        return float64(0), float64(0)
}


func RunGIT_PIR(pi PIR, DB *Database, p Params, i []uint64, metadata [][]uint64, parallel bool) {
	fmt.Printf("Executing %s\n", pi.Name())
	num_queries := uint64(len(i))

	// if DB.Data.Rows/num_queries < DB.Info.Ne {
	// 	panic("Too many queries to handle!")
	// }

	batch_sz := DB.Data.Rows / (DB.Info.Ne * num_queries) * DB.Data.Cols
	shared_state := pi.Init(DB.Info, p)

	fmt.Println("\n(SERVER) DB Setup...")
	start := time.Now()
	server_state, offline_download := pi.Setup(DB, shared_state, p)	
	DB.Data.Dim()

	comm := float64(offline_download.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	fmt.Println("p.Logq:", p.Logq)

	time := printTime(start)
	fmt.Printf("\t\tOffline download: %f KB\n", comm)

	SERVER_DB_SETUP_TIME = time
	SERVER_DB_SETUP_OFFLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"

	runtime.GC()
	
	// ClientServerPIRConcurrently(DB, i, shared_state, server_state, batch_sz, p, pi, offline_download, metadata)
	
	if (parallel) {
		fmt.Println("Run in PARALLEL")
		torPIRConcurrently(DB, i, shared_state, server_state, batch_sz, p, pi, offline_download, metadata)
	} else {
		fmt.Println("Run SEQUENTIALLY")
		ClientServerPIR(DB, i, shared_state, server_state, batch_sz, p, pi, offline_download, metadata)
	}
	return
}

func ClientServerPIR(DB *Database, i []uint64, shared_state State, server_state State, batch_sz uint64, p Params, pi PIR, offline_download Msg, metadata [][]uint64) {
	// fmt.Println("\n\n==> i:", i)

	// CLIENT
	query, client_state := BuildingQuery(DB.Info, i, shared_state, batch_sz, p, pi)
	runtime.GC()

	// SERVER
	answer := AnsweringQuery(DB, query, server_state, shared_state, p, pi)
	runtime.GC()

	// CLIENT
	result := ReconstructingAnswer(i, batch_sz, offline_download, query, answer, shared_state, client_state, DB.Info, p, pi, metadata[0])
	runtime.GC()
	
	fmt.Printf("\n\n*************\n\nREQUESTED REPO: %v\n\n*************\n\n", result)
	return
}

func ReconstructingAnswer(i []uint64, batch_sz uint64, offline_download Msg, query MsgSlice, answer Msg, shared_state State, client_state []State, DBinfo DBinfo, p Params, pi PIR, metadata []uint64) uint64 {
	fmt.Println("(CLIENT) Reconstructing...")
	start := time.Now()
	result := uint64(0)
	for index, _ := range i {
		index_to_query := i[index] + uint64(index)*batch_sz
		val := pi.RecoverFile(index_to_query, uint64(index), offline_download, query.Data[index], answer, shared_state, client_state[index], p, DBinfo, metadata)
		result = val
	}
	time := printTime(start)

	CLIENT_RECONSTRUCTION_TIME = time

	return result
}

func AnsweringQuery(DB *Database, query MsgSlice, server_state State, shared_state State, p Params, pi PIR) Msg {
	fmt.Println("(SERVER) Answering query...")
	start := time.Now()
	answer := pi.Answer(DB, query, server_state, shared_state, p)
	time := printTime(start)
	printRate(p, time, 1)
	comm := float64(answer.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	fmt.Printf("\t\tOnline download: %f KB\n", comm)
	pi.Reset(DB, p)

	SERVER_ANSWER_QUERY_TIME = time
	SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"

	return answer
}

func BuildingQuery(DBinfo DBinfo, i []uint64, shared_state State, batch_sz uint64, p Params, pi PIR) (MsgSlice, []State) {
	fmt.Println("\n(CLIENT) Building query...")
	var client_state []State
	var query MsgSlice
	start := time.Now()
	for index, _ := range i {
		index_to_query := i[index] + uint64(index)*batch_sz
		cs, q := pi.Query(index_to_query, shared_state, p, DBinfo)
		client_state = append(client_state, cs)
		query.Data = append(query.Data, q)
	}
	time := printTime(start)
	comm := float64(query.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	fmt.Printf("\t\tOnline upload: %f KB\n", comm)

	CLIENT_BUILDING_QUERY = time
	CLIENT_BUILDING_QUERY_ONLINE_UPLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"
	
	return query, client_state
}

///////////////////////////////////////////////

func ClientServerPIRConcurrently(DB *Database, i []uint64, shared_state State, server_state State, batch_sz uint64, p Params, pi PIR, offline_download Msg, metadata [][]uint64) []uint64 {
	chunks := int(metadata[i[0]][1])
	fmt.Println("==> chunks:", chunks)
	// deepCopyDB := deepcopy.Copy(DB).(*Database)
	deepCopyDB := deepCopyDatabase(*DB)

	// CLIENT
	start_query := time.Now()
	comm := float64(0)
	var clientWG sync.WaitGroup
	queries := make([]MsgSlice, chunks)
	client_states := make([][]State, chunks)
	queryChan := make(chan QueryChannel, chunks)

	fmt.Println("\n(CLIENT) Building query...")
	for j := 0; j < chunks; j++ {
		clientWG.Add(1)
		i = []uint64{uint64(j)}
		go BuildingQueryConcurrently(DB.Info, i, shared_state, batch_sz, p, pi, j, queryChan, &clientWG)
	}
	clientWG.Wait()
	close(queryChan)

	for j := 0; j < chunks; j++ {
		q := <-queryChan
		queries[q.Index] = q.Query
		client_states[q.Index] = q.States
		comm += float64(q.Query.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	}
	CLIENT_BUILDING_QUERY = printTime(start_query)
	CLIENT_BUILDING_QUERY_ONLINE_UPLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"
	runtime.GC()

	// SERVER OFFLINE - DB DEEPCOPIES
	deepcopy_time := time.Now()
	dbArray := make([]Database, chunks)
	for i := 0; i < chunks; i++ {
		// dbArray[i] = deepcopy.Copy(deepCopyDB).(*Database)
		deepCopyDBAgain := deepCopyDatabase(deepCopyDB)
		dbArray[i] = deepCopyDBAgain
		// printMemoryUsage()
	}
	fmt.Println("(SERVER) DB Deepcopy...")
	printTime(deepcopy_time)

	// SERVER
	start_answering := time.Now()
	comm = float64(0)
	var serverWG sync.WaitGroup
	answerChan := make(chan AnswerChannel, chunks)
	answers := make([]Msg, chunks)
	fmt.Println("(SERVER) Answering query...")
	for j := 0; j < chunks; j++ {
		serverWG.Add(1)
		go AnsweringQueryConcurrently(&dbArray[j], queries[j], server_state, shared_state, p, pi, j, answerChan, &serverWG)
	}
	serverWG.Wait()
	close(answerChan)
	for j := 0; j < chunks; j++ {
		x := <-answerChan
		answers[x.Index] = x.Answer
		comm += float64(x.Answer.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	}
	SERVER_ANSWER_QUERY_TIME = printTime(start_answering)
	printRate(p, SERVER_ANSWER_QUERY_TIME, 1)

	SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"
	runtime.GC()

	// CLIENT
	reconstructingAnswerChan := make(chan ReconstructingAnswerChannel, chunks)
	reconstructedAnswersArr := make([][]uint64, chunks)

	start_reconstruction := time.Now()
	var wg sync.WaitGroup
	fmt.Println("(CLIENT) Reconstructing...")
	for j := 0; j < chunks; j++ {
		wg.Add(1)
		i = []uint64{uint64(j)}
		go ReconstructingAnswerConcurrently(i, batch_sz, offline_download, queries[j], answers[j], shared_state, client_states[j], DB.Info, p, pi, metadata[j], j, reconstructingAnswerChan, &wg)
	}
	wg.Wait()
	close(reconstructingAnswerChan)

	for j := 0; j < chunks; j++ {
		x := <-reconstructingAnswerChan
		reconstructedAnswersArr[x.Index] = x.Answer
		// fmt.Println("\n==> x.Index:", x.Index)
		// fmt.Println(x.Answer)
	}
	ans := flattenArray(reconstructedAnswersArr)
	CLIENT_RECONSTRUCTION_TIME = printTime(start_reconstruction)
	runtime.GC()

	return ans
}

func BuildingQueryConcurrently(DBinfo DBinfo, i []uint64, shared_state State, batch_sz uint64, p Params, pi PIR, index int, queryChan chan QueryChannel, clientWG *sync.WaitGroup) {
	// fmt.Println("==> BuildingQuery")
	defer clientWG.Done()
	var client_state []State
	var query MsgSlice
	// start := time.Now()
	for index_, _ := range i {
		index_to_query := i[index_] + uint64(index_)*batch_sz
		cs, q := pi.Query(index_to_query, shared_state, p, DBinfo)
		client_state = append(client_state, cs)
		query.Data = append(query.Data, q)
	}

	queryChan <- QueryChannel{Index: index, Query: query, States: client_state}

	// time := printTime(start)
	// comm := float64(query.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	// fmt.Printf("\t\tOnline upload: %f KB\n", comm)

	// CLIENT_BUILDING_QUERY = time
	// CLIENT_BUILDING_QUERY_ONLINE_UPLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"
	
	// return query, client_state
}

func ReconstructingAnswerConcurrently(i []uint64, batch_sz uint64, offline_download Msg, query MsgSlice, answer Msg, shared_state State, client_state []State, DBinfo DBinfo, p Params, pi PIR, metadata []uint64, db_index int,reconstructingAnswerChan chan ReconstructingAnswerChannel, wg *sync.WaitGroup) {
	// fmt.Println("==> ReconstructingAnswer")
	defer wg.Done()
	var vals []uint64

	for index, _ := range i {
		index_to_query := i[index] + uint64(index)*batch_sz
		vals = pi.RecoverRepository(index_to_query, uint64(index), offline_download, query.Data[index], answer, shared_state, client_state[index], p, DBinfo, metadata)
	}
	reconstructingAnswerChan <- ReconstructingAnswerChannel{Index: db_index, Answer: vals}

}

func AnsweringQueryConcurrently(DB *Database, query MsgSlice, server_state State, shared_state State, p Params, pi PIR, index int, answerChan chan AnswerChannel, serverWG *sync.WaitGroup) {
	// fmt.Println("==> AnsweringQuery")
	defer serverWG.Done()
	answer := pi.Answer(DB, query, server_state, shared_state, p)
	pi.Reset(DB, p)
	// answerChan <- answer
	answerChan <- AnswerChannel{Index: index, Answer: answer}
}

///////////////////////////////////////////////

func torPIRConcurrently(DB *Database, i []uint64, shared_state State, server_state State, batch_sz uint64, p Params, pi PIR, offline_download Msg, metadata [][]uint64) {
	deepCopyDB := deepCopyDatabase(*DB)
	previousChunk := 0
	for j, repo_index := range REPO_INDEXES {
		fmt.Println("==> repo_index:", repo_index)
		INDEX = j
		CHUNKS = int(metadata[repo_index][1])

		PADDED_REPO_SIZE = int((metadata[repo_index][2]) * (metadata[repo_index][1]))
		ORIGINAL_REPO_SIZE = 0
		for i := REPO_INDEXES[j]; i < REPO_INDEXES[j] + CHUNKS; i++ {
			ORIGINAL_REPO_SIZE += int(metadata[i][2])
		}

		if (previousChunk == CHUNKS){
			fmt.Println("!!!SAME CHUNK SIZE!!! INDEX:", INDEX, "repo_index:", repo_index, "CHUNKS:", CHUNKS)
			writeToCSV()
			continue
		}

		fmt.Println("\n==> INDEX:", INDEX, "repo_index:", repo_index, "CHUNKS:", CHUNKS)

		i[0] = uint64(repo_index)
		DB := deepCopyDatabase(deepCopyDB)

		RESULTS = [][]uint64{}
		
		result_bytes := ClientServerPIRConcurrently(&DB, i, shared_state, server_state, batch_sz, p, pi, offline_download, metadata)

		// fmt.Println("==> PADDED_REPO_SIZE:", PADDED_REPO_SIZE, "ORIGINAL_REPO_SIZE:", ORIGINAL_REPO_SIZE)
		fmt.Println("==> result_bytes      :", len(result_bytes))
		
		writeToCSV()
		previousChunk = CHUNKS
		break
	}
}

///////////////////////////////////////////////

func RunGIT_PIR_Multiserver_Multi_Request(pi PIR, DB []*Database, p Params, repos [][]uint64, fileSizes []int, parallel bool) {

	fmt.Println("=!=!= RunGIT_PIR_Multiserver_Multi_Request =!=!=")
	fmt.Printf("Executing %s\n", pi.Name())
	numDBs := len(DB)

	batch_sz := DB[0].Data.Rows / (DB[0].Info.Ne * 1) * DB[0].Data.Cols

	fmt.Println("\n(SERVER) DB Setup...")
	start := time.Now()

	shared_state := make([]State, numDBs)
	offline_download := make([]Msg, numDBs)
	server_state := make([]State, numDBs)

	for i := 0; i < numDBs; i++ {
		_shared_state := pi.Init(DB[i].Info, p)
		shared_state[i] = _shared_state

		_server_state, _offline_download := pi.Setup(DB[i], shared_state[i], p)
		server_state[i] = _server_state
		offline_download[i] = _offline_download
	}

	DB[0].Data.Dim()	

	comm := float64(uint64(numDBs) * offline_download[0].Size() * uint64(p.Logq) / (8.0 * 1024.0))
	fmt.Println("p.Logq:", p.Logq)

	time := printTime(start)
	fmt.Printf("\t\tOffline download: %f KB\n", comm)

	SERVER_DB_SETUP_TIME = time
	SERVER_DB_SETUP_OFFLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"

	runtime.GC()

	for repo_index := 0; repo_index < len(repos); repo_index++ {
		_shared_state := deepcopy.Copy(shared_state).([]State)
		_server_state := deepcopy.Copy(server_state).([]State)
		_DB := deepcopy.Copy(DB).([]*Database)

		ans := []uint64{}
		i := []uint64{uint64(repo_index)}

		if (parallel) {
			ans = torPIRMultiserver(_DB, i, _shared_state, _server_state, batch_sz, p, pi, offline_download)
		} else {
			ans = torPIRMultiserverSequentially(_DB, i, _shared_state, _server_state, batch_sz, p, pi, offline_download)
		}

		if (areEqual(repos[repo_index][:fileSizes[repo_index]], ans[:fileSizes[repo_index]]) && calculateChecksum(repos[repo_index][:fileSizes[repo_index]]) == calculateChecksum(ans[:fileSizes[repo_index]])) {
			fmt.Println("\n\n********************************\n\n!!! SUCCESS !!!\n\n********************************\n ")
			SICES_SAME = 1
		} else {
			fmt.Println("\n\n!!!!****!!!!****!!!!****!!!!****\n\n!!! FALIURE !!!\n\n!!!!****!!!!****!!!!****!!!!****\n ")
			SICES_SAME = 0
		}

		runtime.GC()
	}

}

///////////////////////////////////////////////

func RunGIT_PIR_Multiserver(pi PIR, DB []*Database, p Params, i []uint64, parallel bool) []uint64 {

	fmt.Println("=!=!= RunGIT_PIR_Multiserver =!=!=")
	fmt.Printf("Executing %s\n", pi.Name())
	numDBs := len(DB)
	num_queries := uint64(len(i))

	batch_sz := DB[0].Data.Rows / (DB[0].Info.Ne * num_queries) * DB[0].Data.Cols

	fmt.Println("\n(SERVER) DB Setup...")
	start := time.Now()

	shared_state := make([]State, numDBs)
	offline_download := make([]Msg, numDBs)
	server_state := make([]State, numDBs)

	for i := 0; i < numDBs; i++ {
		shared_state[i] = pi.Init(DB[0].Info, p)

		_server_state, _offline_download := pi.Setup(DB[i], shared_state[i], p)

		if (i == 0){
			offline_download[0] = _offline_download
		}
		server_state[i] = _server_state
		offline_download[i] = _offline_download
	}

	DB[0].Data.Dim()

	// os.Exit(1)
	
	comm := float64(uint64(numDBs) * offline_download[0].Size() * uint64(p.Logq) / (8.0 * 1024.0))

	time := printTime(start)
	fmt.Printf("\t\tOffline download: %f KB\n", comm)

	SERVER_DB_SETUP_TIME = time
	SERVER_DB_SETUP_OFFLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"

	runtime.GC()

	if (parallel) {
		return torPIRMultiserver(DB, i, shared_state, server_state, batch_sz, p, pi, offline_download)
	}

	return torPIRMultiserverSequentially(DB, i, shared_state, server_state, batch_sz, p, pi, offline_download)
}

func torPIRMultiserverSequentially(DB []*Database, i []uint64, shared_state []State, server_state []State, batch_sz uint64, p Params, pi PIR, offline_download []Msg) []uint64 {
	numDBs := len(DB)

	// CLIENT
	start_query := time.Now()
	comm := float64(0)
	queries := make([]MsgSlice, numDBs)
	client_states := make([][]State, numDBs)

	fmt.Println("\n(CLIENT) Building query...")
	for j := 0; j < numDBs; j++ {
		var client_state []State
		var query MsgSlice

		cs, q := pi.Query(i[0], shared_state[j], p, DB[j].Info)
		client_state = append(client_state, cs)
		query.Data = append(query.Data, q)

		queries[j] = query
		client_states[j] = client_state
		comm += float64(query.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	}
	
	CLIENT_BUILDING_QUERY = printTime(start_query)
	CLIENT_BUILDING_QUERY_ONLINE_UPLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"
	runtime.GC()


	// SERVER
	start_answering := time.Now()
	comm = float64(0)
	answers := make([]Msg, numDBs)
	fmt.Println("(SERVER) Answering query...")

	for j := 0; j < numDBs; j++ {
		answer := pi.Answer(DB[j], queries[j], server_state[j], shared_state[j], p)
		pi.Reset(DB[j], p)

		answers[j] = answer
		comm += float64(answer.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	}

	SERVER_ANSWER_QUERY_TIME = printTime(start_answering)
	printRate(p, SERVER_ANSWER_QUERY_TIME, 1)
	SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"
	runtime.GC()


	// CLIENT
	reconstructedAnswersArr := make([][]uint64, numDBs)
	start_reconstruction := time.Now()
	fmt.Println("(CLIENT) Reconstructing...")
	for j := 0; j < numDBs; j++ {
		var vals []uint64
		vals = pi.RecoverRepository(i[0], uint64(0), offline_download[j], queries[j].Data[0], answers[j], shared_state[j], client_states[j][0], p, DB[j].Info, []uint64{0})
		reconstructedAnswersArr[j] = vals
	}
	
	ans := flattenArray(reconstructedAnswersArr)
	CLIENT_RECONSTRUCTION_TIME = printTime(start_reconstruction)
	runtime.GC()

	writeToCSV()

	return ans
}

func torPIRMultiserver(DB []*Database, i []uint64, shared_state []State, server_state []State, batch_sz uint64, p Params, pi PIR, offline_download []Msg) []uint64 {
	numDBs := len(DB)

	// CLIENT
	start_query := time.Now()
	comm := float64(0)
	var clientWG sync.WaitGroup
	queries := make([]MsgSlice, numDBs)
	client_states := make([][]State, numDBs)
	queryChan := make(chan QueryChannel, numDBs)

	fmt.Println("\n(CLIENT) Building query...")
	for j := 0; j < numDBs; j++ {
		clientWG.Add(1)
		// i = []uint64{uint64(j)}
		go BuildingQueryConcurrently(DB[j].Info, i, shared_state[j], batch_sz, p, pi, j, queryChan, &clientWG)
	}
	clientWG.Wait()
	close(queryChan)

	for j := 0; j < numDBs; j++ {
		q := <-queryChan
		queries[q.Index] = q.Query
		client_states[q.Index] = q.States
		comm += float64(q.Query.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	}
	CLIENT_BUILDING_QUERY = printTime(start_query)
	CLIENT_BUILDING_QUERY_ONLINE_UPLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"
	runtime.GC()


	// SERVER
	start_answering := time.Now()
	comm = float64(0)
	var serverWG sync.WaitGroup
	answerChan := make(chan AnswerChannel, numDBs)
	answers := make([]Msg, numDBs)
	fmt.Println("(SERVER) Answering query...")
	for j := 0; j < numDBs; j++ {
		serverWG.Add(1)
		go AnsweringQueryConcurrently(DB[j], queries[j], server_state[j], shared_state[j], p, pi, j, answerChan, &serverWG)
	}
	serverWG.Wait()
	close(answerChan)
	for j := 0; j < numDBs; j++ {
		x := <-answerChan
		answers[x.Index] = x.Answer
		comm += float64(x.Answer.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	}

	SERVER_ANSWER_QUERY_TIME = printTime(start_answering)
	printRate(p, SERVER_ANSWER_QUERY_TIME, 1)

	SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"
	runtime.GC()


	// CLIENT
	reconstructingAnswerChan := make(chan ReconstructingAnswerChannel, numDBs)
	reconstructedAnswersArr := make([][]uint64, numDBs)

	start_reconstruction := time.Now()
	var wg sync.WaitGroup
	fmt.Println("(CLIENT) Reconstructing...")
	for j := 0; j < numDBs; j++ {
		wg.Add(1)
		go ReconstructingAnswerConcurrently(i, batch_sz, offline_download[j], queries[j], answers[j], shared_state[j], client_states[j], DB[j].Info, p, pi, []uint64{0}, j, reconstructingAnswerChan, &wg)
	}
	wg.Wait()
	close(reconstructingAnswerChan)

	for j := 0; j < numDBs; j++ {
		x := <-reconstructingAnswerChan
		reconstructedAnswersArr[x.Index] = x.Answer
		// fmt.Println("\n==> x.Index:", x.Index)
		// fmt.Println(x.Answer)
	}
	ans := flattenArray(reconstructedAnswersArr)
	CLIENT_RECONSTRUCTION_TIME = printTime(start_reconstruction)
	runtime.GC()

	writeToCSV()

	return ans
}

func RunFakePIR(pi PIR, DB *Database, p Params, i []uint64, 
                f *os.File, profile bool) (float64, float64, float64, float64) {
	fmt.Printf("Executing %s\n", pi.Name())
	//fmt.Printf("Memory limit: %d\n", debug.SetMemoryLimit(math.MaxInt64))
	debug.SetGCPercent(-1)

	num_queries := uint64(len(i))
	if DB.Data.Rows/num_queries < DB.Info.Ne {
		panic("Too many queries to handle!")
	}
	shared_state := pi.Init(DB.Info, p)

	fmt.Println("Setup...")
	server_state, bw := pi.FakeSetup(DB, p)
	offline_comm := bw
	runtime.GC()

	fmt.Println("Building query...")
	start := time.Now()
	var query MsgSlice
	for index, _ := range i {
		_, q := pi.Query(i[index], shared_state, p, DB.Info)
		query.Data = append(query.Data, q)
	}
	printTime(start)
	online_comm := float64(query.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	CLIENT_BUILDING_QUERY_ONLINE_UPLOAD = strconv.FormatFloat(online_comm, 'f', -1, 64) + " KB"
	fmt.Printf("\t\tOnline upload: %f KB\n", online_comm)
	bw += online_comm
	runtime.GC()

	fmt.Println("Answering query...")
	if profile {
		pprof.StartCPUProfile(f)
	}
	start = time.Now()
	answer := pi.Answer(DB, query, server_state, shared_state, p)
	elapsed := printTime(start)
	if profile {
		pprof.StopCPUProfile()
	}
	rate := printRate(p, elapsed, len(i))
	online_down := float64(answer.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD = strconv.FormatFloat(online_down, 'f', -1, 64) + " KB"
	fmt.Printf("\t\tOnline download: %f KB\n", online_down)
	bw += online_down
	online_comm += online_down

	runtime.GC()
	debug.SetGCPercent(100)
	pi.Reset(DB, p)

	if offline_comm + online_comm != bw {
		panic("Should not happen!")
	}

	return rate, bw, offline_comm, online_comm
}