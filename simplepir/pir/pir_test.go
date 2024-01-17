package pir

import (
	"encoding/csv"
	"fmt"
	"os"
	"testing"
	// "math/rand"
	"runtime"
	"strconv"
	// "reflect"
)

const LOGQ = uint64(32)
const SEC_PARAM = uint64(1 << 10)

var ROWS = 999
var COLS = 999
var ITERATION = 1

var SERVER_DB_SIZE = ""
var FILE_SIZE = ""
var REPO_SIZE = 1
var CHUNKS = 0
var SERVER_DB_SETUP_TIME = ""
var SERVER_DB_DIMENSION = ""
var SERVER_DB_SETUP_OFFLINE_DOWNLOAD = ""
var CLIENT_BUILDING_QUERY = ""
var CLIENT_BUILDING_QUERY_ONLINE_UPLOAD = ""
var SERVER_ANSWER_QUERY_TIME = ""
var SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD = ""
var CLIENT_RECONSTRUCTION_TIME = ""
var SICES_SAME = 1
var INDEX = 0
var PADDED_REPO_SIZE = 0
var ORIGINAL_REPO_SIZE = 0
var REPO_INDEXES []int
var RATE float64
var FILE_NAME = ""
var NUM_DB = 0
var OPTIONAL_FIELD = "OPTIONAL_FIELD"
var OPTIONAL_FIELD_VALUE = ""
var OPTIONAL_FIELD_2 = "OPTIONAL_FIELD_2"
var OPTIONAL_FIELD_2_VALUE = ""

var RESULTS [][]uint64

var ONE_MB_BITS = 8388608

var QUARTER_MB_BYTES = 262144

var ONE_MB_BYTES= 1048576
var ONE_HUNDRED_KB_BYTES = 102400


////////////////////////////////////////////
////////////////////////////////////////////

func torFiles() []int{
	var data []int

	file, err := os.Open("../tor.csv")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return data
	}
	defer file.Close()

	// Create a CSV reader
	reader := csv.NewReader(file)

	// Read all the records
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return data
	}

	sum := 0
	// Print the 2nd column data
	for i, record := range records {
		if i == 0 {
			continue
		}

		if len(record) >= 2 {
			value, err := strconv.Atoi(record[1])
			if err != nil {
				fmt.Printf("Error converting to integer: %v\n", err)
				continue
			}
			// if value < 53000000 { // 111111111
				data = append(data, value)
				sum += value // in bytes
			// }
		}
	}

	fmt.Println("Total Bytes:", formatWithCommas(sum), "bytes")
	return data
}

func createTorRepos(fileSizes []int, chunk_bytes int) {
	fmt.Println("==> Inside createTorRepos!")
	fmt.Println("==> fileSizes:", fileSizes)

	num_repos := len(fileSizes)
	fmt.Println("==> num_repos:", num_repos)
	fmt.Println("==> chunk_bytes:", chunk_bytes)

	desiredSize := 0
	COLS, desiredSize = findBestCols(num_repos, chunk_bytes)
	fmt.Println("\n\n\n==> COLS Found:", formatWithCommas(COLS))


	var repo_indexes []int
	var db_chunks [][]uint64
	var db_metadata [][]uint64
	index := 0
	for _, fileSize := range fileSizes {
		// if fileSize > chunk_bytes {
			// fmt.Println("==> fileSize:", fileSize, "bytes")
			// originalSize := file

			repo := createDummyFile(fileSize)
			// fmt.Println("==> len(repo):", len(repo))
			// fmt.Println("==> desiredSize:", desiredSize)
			chunks := splitBytesIntoChunks(repo, desiredSize)
			CHUNKS = len(chunks)

			// fmt.Println("==> CHUNKS:", CHUNKS)

			var metadata [][]uint64
		    for i, chunk := range chunks {
				metadata = append(metadata, []uint64{uint64(i), uint64(len(chunks)), uint64(len(chunk))})
			}
			// fmt.Println("==> metadata:", metadata)

			// padding for the last chunk
			originalSize := len(chunks[len(chunks)-1])
		    if desiredSize > originalSize {
				chunks[len(chunks)-1] = append(chunks[len(chunks)-1], make([]uint64, desiredSize-originalSize)...)
			}

			db_metadata = append(db_metadata, metadata...)
			db_chunks = append(db_chunks, chunks...)

			repo_indexes = append(repo_indexes, generateArray(CHUNKS, index)...)
			index++
		// }
	}
	fmt.Println("==> len(db_chunks)   :", len(db_chunks))
	fmt.Println("==> len(repo_indexes):", len(repo_indexes), "(this should be equal above and be #ROWS)")
	fmt.Println("==> repo_indexes     :", repo_indexes)

	REPO_INDEXES = firstOccurrenceIndexes(repo_indexes)
	fmt.Println("==> REPO_INDEXES     :", REPO_INDEXES)

	ROWS = len(db_chunks)
	
	N := uint64(ROWS)
	d := uint64(COLS)

	pir := SimplePIR{}
	p := pir.PickParams(N, d, SEC_PARAM, LOGQ)
	
	desiredSize = int(p.L)

	vals := make([]uint64, ROWS)
	for i := 0; i < ROWS; i++ {
		vals[i] = 150
	}

	DB := MakeDB(N, d, &p, vals, db_chunks)
	DB.Data.Dim()

	RunGIT_PIR(&pir, DB, p, []uint64{0}, db_metadata, false)
}

///////////////////////////////////////////

func TestTor(t *testing.T) {
	t.Skip("Skipping...")
	torFileSizesInBytes := torFiles()
	createTorRepos(torFileSizesInBytes, ONE_HUNDRED_KB_BYTES)
}


////////////////////////////////////////////

func createTorReposWithEqualNumOfChunks(fileSizes []int, chunk_bytes int) {
	num_repos := len(fileSizes)
	fmt.Println("==> num_repos:", num_repos)
	fmt.Println("==> chunk_bytes:", chunk_bytes)

	desiredSize := 0
	COLS, desiredSize = findBestCols(num_repos, chunk_bytes)
	fmt.Println("\n\n\n==> COLS Found:", formatWithCommas(COLS))


	var repo_indexes []int
	var db_chunks [][]uint64
	var db_metadata [][]uint64
	index := 0

	maxSize := findMax(fileSizes)
	fmt.Println("==> maxSize:", maxSize)
	findTorFile(maxSize)
	numChunks := (maxSize + desiredSize - 1) / desiredSize // number of chunks calculated wrt max repo size
	fmt.Println("==> numChunks:", numChunks)

	for _, fileSize := range fileSizes {

		repo := createDummyFile(fileSize)
		
		chunks := splitBytesIntoNumChunks(repo, desiredSize, numChunks)

		CHUNKS = len(chunks)

		var metadata [][]uint64
	    for i, chunk := range chunks {
			metadata = append(metadata, []uint64{uint64(i), uint64(len(chunks)), uint64(len(chunk))})
		}

		// padding for the last chunk
		originalSize := len(chunks[len(chunks)-1])
	    if desiredSize > originalSize {
			chunks[len(chunks)-1] = append(chunks[len(chunks)-1], make([]uint64, desiredSize-originalSize)...)
		}

		db_metadata = append(db_metadata, metadata...)
		db_chunks = append(db_chunks, chunks...)

		repo_indexes = append(repo_indexes, generateArray(CHUNKS, index)...)
		index++
	}

	fmt.Println("==> len(db_chunks)   :", len(db_chunks))
	fmt.Println("==> len(repo_indexes):", len(repo_indexes), "(this should be equal above and be #ROWS)")

	REPO_INDEXES = firstOccurrenceIndexes(repo_indexes)
	// fmt.Println("==> REPO_INDEXES     :", REPO_INDEXES)

	ROWS = len(db_chunks)
	
	fmt.Println("==> ROWS     :", ROWS)
	fmt.Println("==> COLS     :", COLS)


	N := uint64(ROWS)
	d := uint64(COLS)

	pir := SimplePIR{}
	p := pir.PickParams(N, d, SEC_PARAM, LOGQ)
	
	desiredSize = int(p.L)

	vals := make([]uint64, ROWS)
	for i := 0; i < ROWS; i++ {
		vals[i] = 150
	}

	DB := MakeDB(N, d, &p, vals, db_chunks)
	DB.Data.Dim()

	RunGIT_PIR(&pir, DB, p, []uint64{0}, db_metadata, true)
}

////////////////////////////////////////////

func TestTorReposWithEqualNumOfChunks(t *testing.T) {
	// t.Skip("Skipping...")

	FILE_NAME = "results_tor_repos_with_equal_chunk_size"
	OPTIONAL_FIELD = "CHUNK_SIZE (MB)"

	torFileSizesInBytes := torFiles()

	// for ITERATION = 1; ITERATION <= 10; ITERATION++ {
		for _, multiplier := range []float64{0.1, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2.0, 2.25, 2.50, 2.75, 3.0, 3.25, 3.50, 3.75, 4.0, 4.25, 4.50, 4.75, 5.0, 5.25, 5.50} {
		    fmt.Println("==> chunk_size:", multiplier, "MB | ITERATION:", ITERATION)
	        chunk_size := int(float64(ONE_MB_BYTES) * multiplier)
	        OPTIONAL_FIELD_VALUE = strconv.FormatFloat(multiplier, 'f', -1, 32)
	        createTorReposWithEqualNumOfChunks(torFileSizesInBytes[:250], chunk_size)
	        break
	    }
	// }
}

func createTorReposSplitIntoMultiserver(fileSizes []int, chunk_bytes int, parallel bool) {

	requested_index := 2

    repos := makeDummyRepos(fileSizes)
    repos = addPadding(repos)

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

	ans := RunGIT_PIR_Multiserver(&pir, DBs, p, []uint64{uint64(requested_index)}, parallel)
	
	// fmt.Println("==> RESULTS:", (RESULTS))
	// fmt.Println("==> len(ans):", len(ans))
	// fmt.Println("==> ans:\n", ans)
	// fmt.Println("==> ans[:fileSizes[requested_index]]:\n", ans[:fileSizes[requested_index]])
	// fmt.Println("\n==> (ans):\n", (ans))
	// fmt.Println("\n==> len(repos[requested_index]):", len(repos[requested_index]))
	// fmt.Println("==> repos[requested_index]:\n", repos[requested_index])
	// fmt.Println("==> repos[requested_index][:fileSizes[requested_index]]:\n", repos[requested_index][:fileSizes[requested_index]])

	if (areEqual(repos[requested_index][:fileSizes[requested_index]], ans[:fileSizes[requested_index]])) {
		fmt.Println("\n\n********************************\n\n!!! SUCCESS !!!\n\n********************************\n ")
	}

	fmt.Printf("Checksum: %x\n", calculateChecksum(repos[requested_index][:fileSizes[requested_index]]))
	fmt.Printf("Checksum: %x\n", calculateChecksum(ans[:fileSizes[requested_index]]))

	return
}

////////////////////////////////////////////

func TestTorReposSplitIntoMultiDB(t *testing.T) {
	// t.Skip("Skipping...")

	FILE_NAME = "results_tor_repos_split_into_multi_db_parallel"
	OPTIONAL_FIELD = "CHUNK_SIZE (MB)"

	torFileSizesInBytes := torFiles()

	for ITERATION = 1; ITERATION <= 10; ITERATION++ {
		for _, multiplier := range []float64{0.1, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2.0, 2.25, 2.50, 2.75, 3.0, 3.25, 3.50, 3.75, 4.0, 4.25, 4.50, 4.75, 5.0, 5.25, 5.50} {
		    fmt.Println("==> chunk_size:", multiplier, "MB | ITERATION:", ITERATION)
	        chunk_size := int(float64(ONE_MB_BYTES) * multiplier)
	        OPTIONAL_FIELD_VALUE = strconv.FormatFloat(multiplier, 'f', -1, 32)
	        createTorReposSplitIntoMultiserver(torFileSizesInBytes[:333], chunk_size, true)
	        break
	    }
	}
}

////////////////////////////////////////////
func createTorReposSplitIntoMultiserverForSingleChunkSize(fileSizes []int, chunk_bytes int, parallel bool) {

    repos := makeDummyRepos(fileSizes)
    repos = addPadding(repos)

	desiredSize := 0
	num_repos := len(repos)
	COLS, desiredSize = findBestCols(num_repos, chunk_bytes)
	ROWS = num_repos

	N := uint64(ROWS)
	d := uint64(COLS)

	pir := SimplePIR{}
	p := pir.PickParams(N, d, SEC_PARAM, LOGQ)
	desiredSize = int(p.L)

	result := splitIntoChunks(repos, desiredSize)
    db_arr := transposeAndConvertToDBArrays(result)
	numDBs := len(db_arr)
	NUM_DB = numDBs

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

	RunGIT_PIR_Multiserver_Multi_Request(&pir, DBs, p, repos, fileSizes, parallel)
	
	return
}

func TestTorReposSplitIntoMultiDBForSingleChunkSize(t *testing.T) {
	// t.Skip("Skipping...")

	FILE_NAME = "results_tor_repos_split_into_multiserver_for_single_chunk_size"
	OPTIONAL_FIELD = "CHUNK_SIZE (MB)"

	torFileSizesInBytes := torFiles()

	for ITERATION = 1; ITERATION <= 10; ITERATION++ {
	    multiplier := 0.25
	    fmt.Println("==> chunk_size:", multiplier, "MB | ITERATION:", ITERATION)
        chunk_size := int(float64(ONE_MB_BYTES) * multiplier)
        OPTIONAL_FIELD_VALUE = strconv.FormatFloat(multiplier, 'f', -1, 32)
        createTorReposSplitIntoMultiserverForSingleChunkSize(torFileSizesInBytes[:333], chunk_size, true)
        // break
    }
}

////////////////////////////////////////////

func TestTorReposSplitIntoMultiDBSequentially(t *testing.T) {
	// t.Skip("Skipping...")

	FILE_NAME = "results_tor_repos_split_into_multi_db_sequentially"
	OPTIONAL_FIELD = "CHUNK_SIZE (MB)"

	torFileSizesInBytes := torFiles()

	for ITERATION = 1; ITERATION <= 10; ITERATION++ {
		for _, multiplier := range []float64{0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2.0, 2.25, 2.50, 2.75, 3.0, 3.25, 3.50, 3.75, 4.0, 4.25, 4.50, 4.75, 5.0, 5.25, 5.50} {
		    // multiplier = 5.0
		    fmt.Println("==> chunk_size:", multiplier, "MB | ITERATION:", ITERATION)
		    chunk_size := int(float64(ONE_MB_BYTES) * multiplier)
		    OPTIONAL_FIELD_VALUE = strconv.FormatFloat(multiplier, 'f', -1, 32)
		    createTorReposSplitIntoMultiserver(torFileSizesInBytes[:333], chunk_size, false)
		    // break
	    }
	}

}

////////////////////////////////////////////

func TestSimplePIR(t *testing.T) {
	// t.Skip("Skipping...")
	
	FILE_NAME = "results_simplePIR"

	numCPUs := runtime.GOMAXPROCS(0)
	fmt.Println("\nNumber of CPUs: ", numCPUs)

	for REPO_SIZE = 1; REPO_SIZE <= 1; REPO_SIZE *= 10 {
		for ROWS = 10; ROWS <= 10000000000; ROWS *= 10 {
			for COLS = 10; COLS <= 10000000000; COLS = 10 * COLS {
				for ITERATION = 1; ITERATION <= 1; ITERATION++ {
					// if (ROWS * COLS >= 100000000000){
					// 	continue
					// }

					SICES_SAME = 1

					repo := createDummyFile(1)
					// repo := createDummyFile(REPO_SIZE * (1024.0 * 1024.0))
					// fmt.Println("==> repo:", len(repo))
					vals := make([]uint64, ROWS)
					for i := 0; i < ROWS; i++ {
						vals[i] = 150
					}

					N := uint64(ROWS)
					d := uint64(COLS)

					fmt.Println("..\n..\n..\n\n********* ", "ROWS:", ROWS, ", COLS:", COLS, ", REPO_SIZE:", REPO_SIZE, "MB | ITERATION:", ITERATION, " *********")
					fmt.Println("")

					pir := SimplePIR{}

					fmt.Println("==> SEC_PARAM:", SEC_PARAM)

					p := pir.PickParams(N, d, SEC_PARAM, LOGQ)

					originalSize := len(repo)
					desiredSize := int(p.L)

					fmt.Println("==> DB height (COLS):", formatWithCommas(COLS))
					fmt.Println("==> DB height (p.L) :", formatWithCommas(int(p.L)))
					fmt.Println("==> DB height (Diff):", formatWithCommas(COLS - desiredSize))
					fmt.Println("==> DB width        :", formatWithCommas(int(p.M)))


					chunks := splitBytesIntoChunks(repo, desiredSize)
					CHUNKS = len(chunks)

					if CHUNKS > 1000 {
						continue
					}
					fmt.Println("==> CHUNKS:", CHUNKS)

					// if CHUNKS == 1 {
					// 	multiplier = 10000000
					// }

					fmt.Println("==> repo:", len(repo))
					fmt.Println("==> desiredSize:", (desiredSize))

				    var metadata [][]uint64
				    for i, chunk := range chunks {
						metadata = append(metadata, []uint64{uint64(i), uint64(len(chunks)), uint64(len(chunk))})
					}

					originalSize = len(chunks[len(chunks)-1])
				    if desiredSize > originalSize {
						chunks[len(chunks)-1] = append(chunks[len(chunks)-1], make([]uint64, desiredSize-originalSize)...)
					}

					// build rest of the database with empty bytes
					if int(ROWS) > len(chunks) {
						for i := len(chunks); i < int(N); i++ {
					        newChunk := make([]uint64, desiredSize)
							chunks = append(chunks, newChunk)
				    	}
				    } else {
				    	continue
				    }

					if ROWS < len(chunks) {
						fmt.Println("Rows are less than chunks")
						continue
					}

					fmt.Println("==> len(chunks):", len(chunks), "(this should equal ROWS)")


					DB := MakeDB(N, d, &p, vals, chunks)
					DB.Data.Dim()

					RunGIT_PIR(&pir, DB, p, []uint64{0}, metadata, false)

					reconstructedBytes := reconstructBytes(RESULTS, ROWS)
					
					checkTwoArrays(repo, reconstructedBytes)
					// fmt.Println("==> SICES_SAME:", SICES_SAME)

					writeToCSV()
					runtime.GC()
				}
			}
		}
	}
}

// Benchmark SimplePIR performance.
func TestBenchmarkSimplePirSingle(t *testing.T) {
	// t.Skip("Skipping...")

	FILE_NAME = "results_simplePIR_benchmarks"
	OPTIONAL_FIELD = "ONLINE_COMM"

	f, err := os.Create("simple-cpu.out")
	if err != nil {
		panic("Error creating file")
	}

	N := uint64(100) 	// ROWS (number of entries)
	d := uint64(1000000) 				// COLS (entry size)


	for ROWS = 10; ROWS <= 10000000000000000; ROWS *= 10 {
		for COLS = 1; COLS <= 10000000000000000; COLS *= 10 {
			for iteration := 0; iteration < 10; iteration++ {
				if (ROWS * COLS == 10000000000) { 				// 10000000000 = 1GB
					fmt.Println("==> ROWS: ", ROWS, "COLS: ", COLS)

					// N = uint64(ROWS)	// offline download
					// d = uint64(COLS)

					pir := SimplePIR{}
					p := pir.PickParams(N, d, SEC_PARAM, LOGQ)

					i := uint64(0) // index to query
					if i >= p.L*p.M {
						panic("Index out of dimensions")
					}

					DB := MakeRandomDB(N, d, &p)

					throughput, _, offline_comm, online_comm := RunFakePIR(&pir, DB, p, []uint64{i}, f, false)

					RATE = throughput
					OPTIONAL_FIELD_VALUE = strconv.FormatFloat(online_comm, 'f', -1, 64) + " KB"
					SERVER_DB_SETUP_OFFLINE_DOWNLOAD = strconv.FormatFloat(offline_comm, 'f', -1, 64) + " KB"

					fmt.Println("==> throughput:  ", throughput)
					fmt.Println("==> offline_comm:", SERVER_DB_SETUP_OFFLINE_DOWNLOAD)
					fmt.Println("==> online_comm: ", OPTIONAL_FIELD_VALUE)

					writeToCSV()
				}
			}
		}
	}
}
