package pir

import (
	"fmt"
	"testing"
	"strconv"
	"time"
	"runtime"
	"encoding/csv"
	"os"
	"net"
	"encoding/gob"
	"sync"
)

// var PublicDB := []*Database

type LookupTable struct {
	RepoName     string
	DBIndex      int
	RepoSize     int
	Checksum     uint32
}

func makeBinaryRepos(numRepos int) ([][]uint64, []LookupTable) {
 	lookupTable := make([]LookupTable, numRepos)
 	basePath := "/Users/shizaali/Documents/Arsalan/Selected Repos/"

 	var result [][]uint64

	file, err := os.Open("../tor.csv")
	if err != nil {
		fmt.Println("Error opening file:", err)
		os.Exit(1)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		os.Exit(1)
	}

	counter := 0
	for i, record := range records {
		if i == 0 {
			continue
		}

		if len(record) >= 2 && counter < numRepos{
			repoName := record[0]
			filePath := basePath + repoName
			// fmt.Printf("repoName: %v\n", repoName)
			bytesArray := convertRepoToBytes(filePath)
			result = append(result, bytesArray)
			lookupTable[counter] = LookupTable{
				RepoName:   repoName,
				RepoSize:   len(bytesArray),
				Checksum:	calculateChecksum(bytesArray),
				DBIndex:	counter,
			}
			// if (counter == 2){
			// 	// fmt.Println("==> bytesArray[", counter, "]:", bytesArray)
			// 	fmt.Println("==> Checksum[", counter, "]:", calculateChecksum(bytesArray))
			// 	outputPath := "/Users/shizaali/Documents/Arsalan/Clone_Repo/cloned/" + repoName
			// 	convertBytesToRepo(bytesArray, outputPath)
			// 	os.Exit(3)
			// }
			counter++
		}
	}

    return result, lookupTable
}

func TestServerTorReposSplitIntoMultiserver(t *testing.T) {	// Same database size (i.e 1GB), but different chunk sizes.
	// t.Skip("Skipping...")

	FILE_NAME = "results_tor_repos_split_into_multiserver"
	OPTIONAL_FIELD = "CHUNK_SIZE (MB)"

	torFileSizesInBytes := torFiles()

	for ITERATION = 1; ITERATION <= 1; ITERATION++ {
		for _, multiplier := range []float64{0.1, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2.0, 2.25, 2.50, 2.75, 3.0, 3.25, 3.50, 3.75, 4.0, 4.25, 4.50, 4.75, 5.0, 5.25, 5.50} {
		    fmt.Println("==> chunk_size:", multiplier, "MB | ITERATION:", ITERATION)
	        chunk_size := int(float64(ONE_MB_BYTES) * multiplier)
	        OPTIONAL_FIELD_VALUE = strconv.FormatFloat(multiplier, 'f', -1, 32)
	        createServerTorReposSplitIntoMultiserver(torFileSizesInBytes[:30], chunk_size, true)
	        break
	    }
	}
}

func createServerTorReposSplitIntoMultiserver(fileSizes []int, chunk_bytes int, parallel bool) {

	requested_index := 2

    // repos := makeDummyRepos(fileSizes)
    // TODO: Open previous directory and for each repository, zip it and convert it to binary
    repos, lookupTable := makeBinaryRepos(30)
	fmt.Println("==> repos:", len(repos))
	fmt.Println("RepoName:", lookupTable[0].RepoName)


	fmt.Println("==> lookupTable[requested_index].RepoSize:", lookupTable[requested_index].RepoSize)

    repos = addPadding(repos)

	fmt.Println("==> len(repos):", len(repos))
	fmt.Println("==> len(repos[0]):", len(repos[0]))
	// fmt.Println("==> repos[requested_index]:", repos[requested_index][:lookupTable[requested_index].RepoSize])
	fmt.Println("==> Checksum:", calculateChecksum(repos[requested_index][:lookupTable[requested_index].RepoSize]))

	desiredSize := 0
	num_repos := len(repos)
	COLS, desiredSize = findBestCols(num_repos, chunk_bytes)
	ROWS = num_repos

	N := uint64(ROWS)
	d := uint64(COLS)

    fmt.Println("=> ROWS :", ROWS)
    fmt.Println("=> COLS :", COLS)

	pir := SimplePIR{}
	p := pir.PickParams(N, d, SEC_PARAM, LOGQ)
	desiredSize = int(p.L)

	result := splitIntoChunks(repos, desiredSize)
    db_arr := transposeAndConvertToDBArrays(result)
	numDBs := len(db_arr)
	NUM_DB = numDBs
	
    fmt.Println("\n!! => RESULTS (Num of DBs)   :", len(db_arr))
    fmt.Println("!! => RESULTS (Num of Repos) :", len(db_arr[0]))
    fmt.Println("!! => RESULTS (Size of Repos):", len(db_arr[0][0]))

	OPTIONAL_FIELD_2 = "TOTAL REPO SIZE RETRIVED (bytes)"
	OPTIONAL_FIELD_2_VALUE = strconv.FormatFloat(float64(numDBs * len(db_arr[0][0])), 'f', -1, 32)
    fmt.Println("!! => TOTAL REPO SIZE RETRIVED:", numDBs * len(db_arr[0][0]), "bytes")

	fmt.Println("\n==> chunk_bytes:", chunk_bytes, ", desiredSize:", desiredSize, "\n ")

	vals := make([]uint64, ROWS)
	for i := 0; i < ROWS; i++ {
		vals[i] = 150
	}


	DBs := make([]*Database, numDBs)

	for i := 0; i < numDBs; i++ {
		DBs[i] = MakeDB(N, d, &p, vals, db_arr[i])
		// DBs[i].Data.Dim()
	}

	multiDB(&pir, DBs, p, []uint64{uint64(requested_index)}, lookupTable, parallel)

	// ans := multiDB(&pir, DBs, p, []uint64{uint64(requested_index)}, lookupTable, parallel)
	
	// if (areEqual(repos[requested_index][:fileSizes[requested_index]], ans[:fileSizes[requested_index]])) {
	// 	fmt.Println("\n\n********************************\n\n!!! SUCCESS !!!\n\n********************************\n ")
	// }

	// fmt.Println("Checksum:", calculateChecksum(repos[requested_index][:lookupTable[requested_index].RepoSize]))
	// fmt.Println("Checksum:", calculateChecksum(ans[:lookupTable[requested_index].RepoSize]))

	// outputPath := "/Users/shizaali/Documents/Arsalan/Clone_Repo/cloned/" + lookupTable[requested_index].RepoName
	// convertBytesToRepo(ans[:lookupTable[requested_index].RepoSize], outputPath)
	
	return
}

func multiDB(pi PIR, DB []*Database, p Params, i []uint64, lookupTable []LookupTable, parallel bool) []uint64 {

	fmt.Println("=!=!= multiDB =!=!=")
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
	
	comm := float64(uint64(numDBs) * offline_download[0].Size() * uint64(p.Logq) / (8.0 * 1024.0))

	time := printTime(start)
	fmt.Printf("\t\tOffline download: %f KB\n", comm)

	SERVER_DB_SETUP_TIME = time
	SERVER_DB_SETUP_OFFLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"

	runtime.GC()

	if (parallel) {
		return GITPIR_Multiserver(DB, i, shared_state, server_state, batch_sz, p, pi, offline_download, lookupTable)
	}

	return torPIRMultiserverSequentially(DB, i, shared_state, server_state, batch_sz, p, pi, offline_download)
}

func TestClient(t *testing.T) {
	fmt.Println("Client Running...")

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	var receivedMessage BuildingQueryMetaData
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&receivedMessage); err != nil {
		t.Fatalf("Failed to decode message from server: %v", err)
	}


	fmt.Println("==> receivedMessage.Rows:", receivedMessage.Rows)

	pir := SimplePIR{}
	p := pir.PickParams(uint64(receivedMessage.Rows), uint64(receivedMessage.Cols), SEC_PARAM, LOGQ)

	//////////////////////////////////////
	numDBs := receivedMessage.NumDB

	// CLIENT - Building Query
	start_query := time.Now()
	comm := float64(0)
	var clientWG sync.WaitGroup
	queries := make([]MsgSlice, numDBs)
	client_states := make([][]State, numDBs)
	queryChan := make(chan QueryChannel, numDBs)

	fmt.Println("\n(CLIENT) Building query...")
	for j := 0; j < numDBs; j++ {
		clientWG.Add(1)
		go BuildingQueryConcurrently_(receivedMessage.DBInfo[j], receivedMessage.Index, receivedMessage.SharedState[j], receivedMessage.BatchSize, receivedMessage.Params, &pir, j, queryChan, &clientWG)
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

	answeringQueryMetaData := AnsweringQueryMetaData{
	    Queries: 	queries,
	}

	encoder := gob.NewEncoder(conn)
    if err := encoder.Encode(answeringQueryMetaData); err != nil {
        t.Fatalf("Failed to encode and send AnsweringQueryMetaData: %v", err)
    }

    var answerReconstructionMetaDataReceived AnswerReconstructionMetaData
    if err := decoder.Decode(&answerReconstructionMetaDataReceived); err != nil {
		t.Fatalf("Failed to decode message from server: %v", err)
	}
	//////////////////////////////////////

	// RECONSTRUCTION
	reconstructingAnswerChan := make(chan ReconstructingAnswerChannel, numDBs)
	reconstructedAnswersArr := make([][]uint64, numDBs)

	start_reconstruction := time.Now()
	var wg sync.WaitGroup
	fmt.Println("(CLIENT) Reconstructing...")
	for j := 0; j < numDBs; j++ {
		wg.Add(1)
		go ReconstructingAnswerConcurrently_(receivedMessage.Index, receivedMessage.BatchSize, receivedMessage.OfflineDownload[j], queries[j], answerReconstructionMetaDataReceived.Answers[j], receivedMessage.SharedState[j], client_states[j], receivedMessage.DBInfo[j], receivedMessage.Params, &pir, []uint64{0}, j, reconstructingAnswerChan, &wg)
	}
	wg.Wait()
	close(reconstructingAnswerChan)

	for j := 0; j < numDBs; j++ {
		x := <-reconstructingAnswerChan
		reconstructedAnswersArr[x.Index] = x.Answer
	}

	ans := flattenArray(reconstructedAnswersArr)
	CLIENT_RECONSTRUCTION_TIME = printTime(start_reconstruction)
	runtime.GC()

	requested_index := receivedMessage.Index[0]
	ans = ans[:receivedMessage.LookupTable[requested_index].RepoSize]


	fmt.Println("==> LookupTable Checksum:", receivedMessage.LookupTable[requested_index].Checksum)
	fmt.Println("==> Calculated Checksum :", calculateChecksum(ans))
	// fmt.Println("==> Answer:", ans)

	outputPath := "/Users/shizaali/Documents/Arsalan/Clone_Repo/cloned/" + receivedMessage.LookupTable[requested_index].RepoName
	convertBytesToRepo(ans, outputPath)
	

	// //////////////////////////////////////







	// Print the received message
	// fmt.Printf("Client received: %+v\n", receivedMessage)


	// Send "Hello, World" to the server
	// message := "Hello, World\n"
	// _, err = conn.Write([]byte(message))
	// if err != nil {
	// 	t.Fatalf("Failed to write to server: %v", err)
	// }

	// fmt.Println("Client sent:", message)

}

