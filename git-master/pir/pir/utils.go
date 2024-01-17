package pir

import "math"
import "fmt"
import "archive/zip"
import "bytes"
import "os"
import "path/filepath"
import "reflect"


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
	// fmt.Println("==> msg.Data:", msg.Data)
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

		if !info.IsDir() {
			relPath, _ := filepath.Rel(folderPath, filePath)
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

func convertRepoToBytes() []uint64 {
	fmt.Println("In convertReposToBytes")

	folderPath := "../HelloWorld"

	// Step 1: Zip the folder in memory
	zipBytes, err := zipFolderInMemory(folderPath)
	if err != nil {
		fmt.Println("Error zipping folder:", err)
		return make([]uint64, 1)
	}

	// Step 2: Represent the zip as a matrix (byte slice)
	matrix := make([]byte, len(zipBytes))
	copy(matrix, zipBytes)


	// Step 2.5: Convert byte matrix to int matrix
	intMatrix := make([]uint64, len(matrix))
	for i, val := range matrix {
		intMatrix[i] = uint64(val)
	}
	fmt.Println("intMatrix:", len(matrix))

	return intMatrix
}


func checkTwoArrays(slice1 []uint64, slice2 []uint64) {
	if len(slice1) != len(slice2) {
	    fmt.Println("Slices are different")
	}

	differentIndexes := 0

	for i := 0; i < len(slice1); i++ {
	    if slice1[i] != slice2[i] {
	        differentIndexes++
	    }
	}

	if differentIndexes == 0 {
	    fmt.Println("Slices are the same")
	} else {
	    fmt.Printf("Slices are different. Number of differing indexes: %d\n", differentIndexes)
	}

	frequencyMap1 := createFrequencyMap(slice1)
	frequencyMap2 := createFrequencyMap(slice2)

	if reflect.DeepEqual(frequencyMap1, frequencyMap2) {
		fmt.Println("Slices have the same values and frequencies.")
	} else {
		fmt.Println("Slices have different values or frequencies.")
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

func reconstructBytes(chunks [][]uint64, rows int) []uint64 {
	combined := make([][]uint64, rows)

	for _, chunk := range chunks {
		combined[chunk[0]] = chunk[3 : 3+ int(chunk[2])]
	}
	
	var flattened []uint64
    for _, arr := range combined {
        flattened = append(flattened, arr...)
    }
	
	return flattened
}

func createFrequencyMap(slice []uint64) map[uint64]uint64 {
	frequencyMap := make(map[uint64]uint64)
	for _, value := range slice {
		frequencyMap[value]++
	}
	return frequencyMap
}
