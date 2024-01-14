package pir

import "math"
import "fmt"
import "archive/zip"
import "bytes"
import "os"
import "path/filepath"
import "reflect"
import "strconv"
import "runtime"
import "encoding/csv"
import "math/rand"
import "hash/crc32"
import "encoding/binary"

type State struct {
	Data []*Matrix
}

type CompressedState struct {
	Seed *PRGKey
}

type Msg struct {
	Data []*Matrix
}

func (m *Msg) Size() uint64 {
	sz := uint64(0)
	for _, d := range m.Data {
		sz += d.Size()
	}
	return sz
}

type MsgSlice struct {
	Data []Msg
}

func (m *MsgSlice) Size() uint64 {
	sz := uint64(0)
	for _, d := range m.Data {
		sz += d.Size()
	}
	return sz
}

func MakeState(elems ...*Matrix) State {
	st := State{}
	for _, elem := range elems {
		st.Data = append(st.Data, elem)
	}
	return st
}

func MakeCompressedState(elem *PRGKey) CompressedState {
	st := CompressedState{}
	st.Seed = elem
	return st
}

func MakeMsg(elems ...*Matrix) Msg {
	msg := Msg{}
	for _, elem := range elems {
		msg.Data = append(msg.Data, elem)
	}
	return msg
}

func MakeMsgSlice(elems ...Msg) MsgSlice {
	slice := MsgSlice{}
	for _, elem := range elems {
		slice.Data = append(slice.Data, elem)
	}
	return slice
}

// Returns the i-th elem in the representation of m in base p.
func Base_p(p, m, i uint64) uint64 {
	for j := uint64(0); j < i; j++ {
		m = m / p
	}
	return (m % p)
}

// Returns the element whose base-p decomposition is given by the values in vals
func Reconstruct_from_base_p(p uint64, vals []uint64) uint64 {
	res := uint64(0)
	coeff := uint64(1)
	for _, v := range vals {
		res += coeff * v
		coeff *= p
	}
	// fmt.Println("==> vals:", vals)
	// fmt.Println("==> len(vals):", len(vals))
	// fmt.Println("==> res:", res)
	return res
}

// Returns how many entries in Z_p are needed to represent an element in Z_q
func Compute_num_entries_base_p(p, log_q uint64) uint64 {
	// fmt.Println("==> p:", p)
	log_p := math.Log2(float64(p))
	return uint64(math.Ceil(float64(log_q) / log_p))
}

// Returns how many Z_p elements are needed to represent a database of N entries,
// each consisting of row_length bits.
func Num_DB_entries(N, row_length, p uint64) (uint64, uint64, uint64) {
	if float64(row_length) <= math.Log2(float64(p)) {
		// pack multiple DB entries into a single Z_p elem
		logp := uint64(math.Log2(float64(p)))
		entries_per_elem := logp / row_length
		db_entries := uint64(math.Ceil(float64(N) / float64(entries_per_elem)))
		if db_entries == 0 || db_entries > N {
			fmt.Printf("Num entries is %d; N is %d\n", db_entries, N)
			panic("Should not happen")
		}
		return db_entries, 1, entries_per_elem
	}

	// use multiple Z_p elems to represent a single DB entry
	ne := Compute_num_entries_base_p(p, row_length)
	return N * ne, ne, 0
}

func avg(data []float64) float64 {
	sum := 0.0
	num := 0.0
	for _, elem := range data {
		sum += elem
		num += 1.0
	}
	return sum / num
}

func stddev(data []float64) float64 {
	avg := avg(data)
	sum := 0.0
	num := 0.0
	for _, elem := range data {
		sum += math.Pow(elem-avg, 2)
		num += 1.0
	}
	variance := sum / num // not -1!
	return math.Sqrt(variance)
}


/////////////////////////////////////////////


func zipFolderInMemory(folderPath string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	err := filepath.Walk(folderPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(folderPath, filePath)

		if info.IsDir() {
			// Create a directory entry
			zipWriter.CreateHeader(&zip.FileHeader{
				Name:     relPath + "/",
				Method:   zip.Store,
				Modified: info.ModTime(),
			})
			return nil
		}

		zipFile, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		_, err = zipFile.Write(fileContent)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func convertRepoToBytes(folderPath string) []uint64 {
	// Step 1: Zip the folder in memory
	zipBytes, err := zipFolderInMemory(folderPath)
	if err != nil {
		fmt.Println("Error zipping folder:", err)
		return make([]uint64, 1)
	}

	// Step 2: Represent the zip as a matrix (byte slice)
	matrix := make([]byte, len(zipBytes))
	copy(matrix, zipBytes)

	// Step 3: Convert byte matrix to int matrix
	intMatrix := make([]uint64, len(matrix))
	for i, val := range matrix {
		intMatrix[i] = uint64(val)
	}
	// fmt.Println("intMatrix:", len(matrix))

	return intMatrix
}


func checkTwoArrays(slice1 []uint64, slice2 []uint64) {
	SICES_SAME = 0

	if len(slice1) != len(slice2) {
	    fmt.Println("Slices are different. slice1:", len(slice1), ", slice2:", len(slice2))
	    fmt.Println("Diff:", len(slice1) - len(slice2))

	    SICES_SAME *= 0
	    return
	}

	differentIndexes := 0

	for i := 0; i < len(slice1); i++ {
	    if slice1[i] != slice2[i] {
	        differentIndexes++
	    }
	}

	if differentIndexes == 0 {
	    fmt.Println("Slices are the same")
	    SICES_SAME *= 1
	} else {
	    fmt.Printf("Slices are different. Number of differing indexes: %d\n", differentIndexes)
	    SICES_SAME *= 0
	}

	frequencyMap1 := createFrequencyMap(slice1)
	frequencyMap2 := createFrequencyMap(slice2)

	if reflect.DeepEqual(frequencyMap1, frequencyMap2) {
		fmt.Println("Slices have the same values and frequencies.")
		SICES_SAME *= 1
	} else {
		fmt.Println("Slices have different values or frequencies.")
		SICES_SAME *= 0
	}
}


func splitBytesIntoChunks(input []uint64, chunkSize int) [][]uint64 {
    var chunks [][]uint64

    for i := 0; i < len(input); i += chunkSize {
        end := i + chunkSize
        if end > len(input) {
            end = len(input)
        }
        chunks = append(chunks, input[i:end])
    }

    return chunks
}

func splitAndPad(arr []uint64, chunkSize int) [][]uint64 {
    numChunks := (len(arr) + chunkSize - 1) / chunkSize
    result := make([][]uint64, numChunks)

    for i := 0; i < numChunks; i++ {
        start := i * chunkSize
        end := (i + 1) * chunkSize
        if end > len(arr) {
            end = len(arr)
        }

        chunk := make([]uint64, chunkSize)
        copy(chunk, arr[start:end])

        result[i] = chunk
    }

    return result
}


func splitBytesIntoChunksOfOriginalSize(input []uint64, chunkSize int, targetLength int) [][]uint64 {
	fmt.Println("\n\n!!! In splitBytesIntoChunksOfOriginalSize")

	fmt.Println("==> chunkSize:", chunkSize)
    fmt.Println("==> targetLength:", targetLength)

    fmt.Println("==> len(input) BEFORE:", len(input))
    zeros := make([]uint64, targetLength - len(input))
    input = append(zeros, input...)
    fmt.Println("==> len(input) AFTER:", len(input))
    fmt.Println("==> len(zeros):", len(zeros))

    chunks := splitAndPad(input, chunkSize) // adds padding in the end to create array of max repo size.
	fmt.Println("\n==> total num of chunks:", len(chunks))

    for i := 0; i < len(chunks); i++ {
    	fmt.Println("==> len(chunks) BEFORE:", len(chunks[i]))
        paddingStart := i * chunkSize
		paddingEnd := (chunkSize * len(chunks)) - (chunkSize * (i+1))
        
        padding := make([]uint64, paddingStart)
        padding = append(padding, chunks[i]...)
        padding = append(padding, make([]uint64, paddingEnd)...)
		chunks[i] = padding

		fmt.Println("==> len(chunks) AFTER:", len(chunks[i]))
    }

	return chunks
}

func areEqual(slice1, slice2 []uint64) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	for i := 0; i < len(slice1); i++ {
		if slice1[i] != slice2[i] {
			return false
		}
	}
	return true
}


func addPadding(repos [][]uint64) [][]uint64 {
 	maxLen := 0
    for _, arr := range repos {
        if len(arr) > maxLen {
            maxLen = len(arr)
        }
    }

 	for i := range repos {
        for len(repos[i]) < maxLen {
            repos[i] = append(repos[i], 0)
        }
    }

    return repos
}

func makeDummyRepos(fileSizes []int) [][]uint64 {
 	var result [][]uint64

    for _, size := range fileSizes {
        result = append(result, createDummyFile(size))
    }

    return result
}

func splitIntoChunks(input [][]uint64, chunkSize int) [][][]uint64 {
	numRows := len(input)
	numCols := len(input[0])

	result := make([][][]uint64, numRows)

	for i := 0; i < numRows; i++ {
		for j := 0; j < numCols; j += chunkSize {
			end := j + chunkSize
			if end > numCols {
				end = numCols
			}
			chunk := make([]uint64, chunkSize)
			copy(chunk, input[i][j:end])
			result[i] = append(result[i], chunk)
		}
	}

	return result
}

func transposeAndConvertToDBArrays(arr [][][]uint64) [][][]uint64 {
	rows := len(arr)
	cols := len(arr[0])

	transposed := make([][][]uint64, cols)
	for i := 0; i < cols; i++ {
		transposed[i] = make([][]uint64, rows)
		for j := 0; j < rows; j++ {
			transposed[i][j] = arr[j][i]
			// transposed[i][j] = append([]uint64{uint64(i)}, arr[j][i]...) // adding the db_index
		}
	}

	return transposed
}

func flattenArray(arr [][]uint64) []uint64 {
	// Flatten the reordered array
    flattenedArray := []uint64{}
    for _, subArray := range arr {
        flattenedArray = append(flattenedArray, subArray...)
    }
    return flattenedArray
}

func ReconstructAnswersMultiServer(inputArray [][]uint64) []uint64 {
    // Reorder the sub-arrays based on the first index
    reorderedArray := make([][]uint64, len(inputArray))
    for _, subArray := range inputArray {
        reorderedArray[subArray[0]] = subArray[1:]
    }

    // Flatten the reordered array
    flattenedArray := []uint64{}
    for _, subArray := range reorderedArray {
        flattenedArray = append(flattenedArray, subArray...)
    }

    return flattenedArray
}

func calculateChecksum(uint64Array []uint64) uint32 {
	byteSlice := make([]byte, len(uint64Array)*8)
	for i, v := range uint64Array {
		binary.LittleEndian.PutUint64(byteSlice[i*8:], v)
	}
	return crc32.ChecksumIEEE(byteSlice)
}

func createDummyFile(fileSize int) []uint64 {
    // fileSize := size_in_mb * (1024.0 * 1024.0)
    // fmt.Println("==> dummy repo fileSize:", formatWithCommas(fileSize), "bytes")
    // fmt.Println("==> dummy repo fileSize:", formatWithCommas(fileSize * 8), "bits")
    data := make([]byte, int(fileSize))

    intMatrix := make([]uint64, len(data))
	for i, _ := range data {
		intMatrix[i] = uint64(rand.Intn(201))
	}

	return intMatrix
}

func findBestCols(num_repos, chunk_bytes int) (int, int) {
	i := 0
	desiredSize := 0
	for i = chunk_bytes * 8; i <= chunk_bytes * 100; i+= int(chunk_bytes / 100) {
		N := uint64(num_repos * 100)
		d := uint64(i)

		pir := SimplePIR{}
		p := pir.PickParams(N, d, SEC_PARAM, LOGQ)
		
		desiredSize = int(p.L)
		if (desiredSize > chunk_bytes) {
			// fmt.Println("STOP!!!")
			break
		}
	}

	return i, desiredSize

}

func seperateDBChunks(db_chunks [][]uint64, rows int) [][][]uint64 {
	// db_chunks = [][]uint64{
    //     {1, 2, 3},
    //     {4, 5, 6},
    //     {7, 8, 9},
    //     {10, 11, 12},
    // }

	fmt.Printf("\n\n############################\n\n")

	fmt.Println("==> rows :", (rows))
	fmt.Println("==> len(db_chunks) :", len(db_chunks))

	result := make([][][]uint64, rows)

    for i := 0; i < len(db_chunks); i++ {
		// fmt.Printf("==> db_chunks[%v]: %v \n", i, len(db_chunks[i]))
		// fmt.Println("==> db_chunks", i%rows)
		result[i%rows] = append(result[i%rows], db_chunks[i])
		// fmt.Println("==> result[i/rows]", result[i%rows])
    }

    // fmt.Println("==> len(db_chunks[0])", len(db_chunks[0]))

    // fmt.Println("==> len(result[0])", len(result[0]))
    // fmt.Println("==> len(result[1])", len(result[1]))
    // fmt.Println("==> len(result[2])", len(result[2]))
    // fmt.Println("==> len(result[3])", len(result[3]))

	fmt.Printf("\n\n############################\n\n")

    return result
	// os.Exit(1)
}


func splitBytesIntoNumChunks(input []uint64, chunkSize int, numChunks int) [][]uint64 {
    // tryingthis()
    result := make([][]uint64, numChunks)

    for i := 0; i < numChunks; i++ {
        start := i * chunkSize
        end := (i + 1) * chunkSize

        if end > len(input) {
            end = len(input)
        }

        chunk := make([]uint64, chunkSize)

        for j := start; j < end; j++ {
            chunk[j%chunkSize] = input[j]
        }

        result[i] = chunk
    }

    return result
}

func reconstructBytes(chunks [][]uint64, rows int) []uint64 {
	// defer func() {
	// 	if err := recover(); err != nil {
	// 		fmt.Println("Recovered from panic:", err)
	// 		return
	// 	}
	// }()
	combined := make([][]uint64, rows)

	for _, chunk := range chunks {
		combined[chunk[0]] = chunk[3 : 3+ int(chunk[2])]
	}
	
	var flattened []uint64
    for _, arr := range combined {
        flattened = append(flattened, arr...)
    }
    fmt.Println("==> flattened:", len(flattened))
	
	return flattened
}

func createFrequencyMap(slice []uint64) map[uint64]uint64 {
	frequencyMap := make(map[uint64]uint64)
	for _, value := range slice {
		frequencyMap[value]++
	}
	return frequencyMap
}

func formatWithCommas(num int) string {
	strNum := strconv.Itoa(num)

	// Add commas every three digits from the right
	n := len(strNum)
	commaCount := (n - 1) / 3
	formattedNum := make([]byte, len(strNum)+commaCount)

	for i, j := n-1, len(formattedNum)-1; i >= 0; i, j = i-1, j-1 {
		if (n-i-1)%3 == 0 && i < n-1 {
			formattedNum[j] = ','
			j--
		}
		formattedNum[j] = strNum[i]
	}

	return string(formattedNum)
}

func generateArray(length, value int) []int {
    arr := make([]int, length)
    for i := range arr {
        arr[i] = value
    }
    return arr
}

func addVectors(vectors [][]uint64) []uint64 {
    if len(vectors) == 0 {
        return nil
    }

    // Initialize the result vector with the same length as the input vectors
    result := make([]uint64, len(vectors[0]))

    for _, vector := range vectors {
        for i, element := range vector {
            result[i] += element
        }
    }

    return result
}

func firstOccurrenceIndexes(arr []int) []int {
    indexes := make([]int, 0)
    visited := make(map[int]bool)

    for i, elem := range arr {
        if _, ok := visited[elem]; !ok {
            indexes = append(indexes, i)
            visited[elem] = true
        }
    }

    return indexes
}

func findMax(arr []int) int {
    if len(arr) == 0 {
        return 0
    }

    max := arr[0]

    for _, value := range arr {
        if value > max {
            max = value
        }
    }

    return max
}

func findTorFile(fileBytes int){
	file, err := os.Open("../tor.csv")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)

	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return
	}

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
			if value == fileBytes {
				fmt.Println("==> Repo name:", record[0], "| bytes:", value)
			}
		}
	}
}

func printMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("Allocated memory: %v bytes\n", m.Alloc)
	fmt.Printf("Total allocated memory (including internal structures): %v bytes\n", m.TotalAlloc)
	fmt.Printf("Heap memory allocated: %v bytes\n", m.HeapAlloc)
}