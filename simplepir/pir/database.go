package pir

import (
	// "encoding/csv"
	"fmt"
	"math"
	"os"
	// "strconv"
	// "strings"
	"archive/zip"
	"bytes"
	// "fmt"
	"io"
	// "time"
	"path/filepath"
	"strconv"
)


type DBinfo struct {
	Num        uint64 // number of DB entries.
	Row_length uint64 // number of bits per DB entry.

	Packing uint64 // number of DB entries per Z_p elem, if log(p) > DB entry size.
	Ne      uint64 // number of Z_p elems per DB entry, if DB entry size > log(p).

	X uint64 // tunable param that governs communication,
	         // must be in range [1, ne] and must be a divisor of ne;
	         // represents the number of times the scheme is repeated.
	P    uint64 // plaintext modulus.
	Logq uint64 // (logarithm of) ciphertext modulus.

	// For in-memory DB compression
	Basis     uint64 
	Squishing uint64
	Cols      uint64
}

type Database struct {
	Info DBinfo
	Data *Matrix
}

func (DB *Database) Squish() {
	//fmt.Printf("Original DB dims: ")
	//DB.Data.Dim()

	DB.Info.Basis = 10
	DB.Info.Squishing = 3
	DB.Info.Cols = DB.Data.Cols
	DB.Data.Squish(DB.Info.Basis, DB.Info.Squishing)

	//fmt.Printf("After squishing, with compression factor %d: ", DB.Info.Squishing)
	//DB.Data.Dim()

	// Check that params allow for this compression
	if (DB.Info.P > (1 << DB.Info.Basis)) || (DB.Info.Logq < DB.Info.Basis * DB.Info.Squishing) {
		panic("Bad params")
	}
}


///////////////////////////////////////////// 

func convertBytesToRepo(intMatrix []uint64, outputFolderPath string) {
	byteMatrix := make([]byte, len(intMatrix))
	for i, val := range intMatrix {
		if val < 0 {
			val = 0
		} else if val > 255 {
			val = 255
		}
		byteMatrix[i] = byte(val)
	}
	
	os.Mkdir(outputFolderPath, os.ModePerm)

	zipReader, err := zip.NewReader(bytes.NewReader(byteMatrix), int64(len(byteMatrix)))
	if err != nil {
		fmt.Println("Error creating zip reader:", err)
		return
	}

	for _, zipFile := range zipReader.File {
		extractedPath := filepath.Join(outputFolderPath, zipFile.Name)
		if zipFile.FileInfo().IsDir() {
			os.MkdirAll(extractedPath, os.ModePerm)
			continue
		}

		// Ensure the directory for the extracted file exists
		extractedDir := filepath.Dir(extractedPath)
		os.MkdirAll(extractedDir, os.ModePerm)

		extractedFile, err := os.Create(extractedPath)
		if err != nil {
			fmt.Println("Error creating extracted file:", err)
			return
		}
		defer extractedFile.Close()

		zipFileReader, err := zipFile.Open()
		if err != nil {
			fmt.Println("Error opening zip file:", err)
			return
		}
		defer zipFileReader.Close()

		_, err = io.Copy(extractedFile, zipFileReader)
		if err != nil {
			fmt.Println("Error extracting file:", err)
			return
		}
	}

	fmt.Println("Extraction complete.")
}

////////////////////////////////////////////



func (DB *Database) Unsquish() {
	DB.Data.Unsquish(DB.Info.Basis, DB.Info.Squishing, DB.Info.Cols)
}

// Store the database with entries decomposed into Z_p elements, and mapped to [-p/2, p/2]
// Z_p elements that encode the same database entry are stacked vertically below each other.
func ReconstructElem(vals []uint64, index uint64, info DBinfo) uint64 {
	q := uint64(1 << info.Logq)

	for i, _ := range vals {
		vals[i] = (vals[i] + info.P/2) % q
		vals[i] = vals[i] % info.P
	}

	val := Reconstruct_from_base_p(info.P, vals)

	if info.Packing > 0 {
		val = Base_p((1 << info.Row_length), val, index%info.Packing)
	}

	return val
}


func ReconstructElemRepo(vals []uint64, index uint64, info DBinfo, metadata []uint64) uint64 {
	q := uint64(1 << info.Logq)

	for i, _ := range vals {
		vals[i] = (vals[i] + info.P/2) % q
		vals[i] = vals[i] % info.P
	}

    result_arr := append(metadata, vals...)
	RESULTS = append(RESULTS, result_arr)

	// fmt.Println("==> val:", vals)
	// convertBytesToRepo(vals, index)
	// convertBytesToRepo(vals[:originalSize], index)


	return uint64(0)
}

func (DB *Database) GetElem(i uint64) uint64 {
	if i >= DB.Info.Num {
		panic("Index out of range")
	}

	col := i % DB.Data.Cols
	row := i / DB.Data.Cols

	if DB.Info.Packing > 0 {
		new_i := i / DB.Info.Packing
		col = new_i % DB.Data.Cols
		row = new_i / DB.Data.Cols
	}

	var vals []uint64
	for j := row * DB.Info.Ne; j < (row+1)*DB.Info.Ne; j++ {
		vals = append(vals, DB.Data.Get(j, col))
	}

	return ReconstructElem(vals, i, DB.Info)
}

// Find smallest l, m such that l*m >= N*ne and ne divides l, where ne is
// the number of Z_p elements per DB entry determined by row_length and p.
func ApproxSquareDatabaseDims(N, row_length, p uint64) (uint64, uint64) {
	db_elems, elems_per_entry, _ := Num_DB_entries(N, row_length, p)
	l := uint64(math.Floor(math.Sqrt(float64(db_elems))))

	// fmt.Println("==> db_elems:", db_elems)
	// fmt.Println("==> elems_per_entry:", elems_per_entry)
	// fmt.Println("==> l:", l)

	rem := l % elems_per_entry
	if rem != 0 {
		l += elems_per_entry - rem
	}

	m := uint64(math.Ceil(float64(db_elems) / float64(l)))
	// fmt.Println("==> m:", m)

	return l, m
}

// Find smallest l, m such that l*m >= N*ne and ne divides l, where ne is
// the number of Z_p elements per DB entry determined by row_length and p, and m >=
// lower_bound_m.
func ApproxDatabaseDims(N, row_length, p, lower_bound_m uint64) (uint64, uint64) {
	l, m := ApproxSquareDatabaseDims(N, row_length, p)
	if m >= lower_bound_m {
		return l, m
	}

	m = lower_bound_m
	db_elems, elems_per_entry, _ := Num_DB_entries(N, row_length, p)
	l = uint64(math.Ceil(float64(db_elems) / float64(m)))

	rem := l % elems_per_entry
	if rem != 0 {
		l += elems_per_entry - rem
	}

	return l, m
}

func SetupDB(Num, row_length uint64, p *Params) *Database {
	if (Num == 0) || (row_length == 0) {
		panic("Empty database!")
	}

	D := new(Database)

	D.Info.Num = Num
	D.Info.Row_length = row_length
	D.Info.P = p.P
	D.Info.Logq = p.Logq

	db_elems, elems_per_entry, entries_per_elem := Num_DB_entries(Num, row_length, p.P)
	D.Info.Ne = elems_per_entry
	D.Info.X = D.Info.Ne
	D.Info.Packing = entries_per_elem

	for D.Info.Ne%D.Info.X != 0 {
		D.Info.X += 1
	}

	D.Info.Basis = 0
	D.Info.Squishing = 0

	DB_size := float64(p.L*p.M)*math.Log2(float64(p.P))/(1024.0*1024.0*8.0)
	
	// fmt.Println("size::", (float64(p.L*p.M)/(1024.0*1024.0*8.0)))
	// fmt.Println("DB_size::", DB_size)

	// fmt.Println("p.L:", float64(p.L))
	// fmt.Println("p.M:", float64(p.M))
	// fmt.Println("p.L*p.M:", float64(p.L*p.M))
	// fmt.Println("p.P:", p.P)
	// fmt.Println("log p.P:", math.Log2(float64(p.P)))
	// fmt.Println("1024.0*1024.0*8.0:", float64(1024.0*1024.0*8.0))
	// fmt.Println("math.Log2(float64(p.P))/(1024.0*1024.0*8.0):", float64(math.Log2(float64(p.P))/(1024.0*1024.0*8.0)))


	fmt.Println("Size of single file:", strconv.FormatFloat(float64(DB_size) / float64(Num), 'f', 5, 64), "MB")

	fmt.Printf("Total packed DB size is ~%f MB\n", DB_size)

	SERVER_DB_SIZE = strconv.FormatFloat(DB_size, 'f', 5, 64) + " MB"
	FILE_SIZE = strconv.FormatFloat(DB_size / float64(ROWS), 'f', 5, 64) + " MB"

	if db_elems > p.L*p.M {
		panic("Params and database size don't match")
	}

	if p.L%D.Info.Ne != 0 {
		panic("Number of DB elems per entry must divide DB height")
	}

	return D
}

func MakeRandomDB(Num, row_length uint64, p *Params) *Database {
	D := SetupDB(Num, row_length, p)
	D.Data = MatrixRand(p.L, p.M, 0, p.P)

	// Map DB elems to [-p/2; p/2]
	D.Data.Sub(p.P / 2)

	return D
}

func MakeDB(Num, row_length uint64, p *Params, vals []uint64, repo [][]uint64) *Database {
	D := SetupDB(Num, row_length, p)
	D.Data = MatrixZeros(p.L, p.M)
	if uint64(len(vals)) != Num {
		panic("Bad input DB")
	}

	// fmt.Println("==> D.Info.Ne:", D.Info.Ne)
	// fmt.Println("==> len(repo):", len(repo[0]))
	for i, _ := range vals {
		for j := uint64(0); j < D.Info.Ne; j++ {
			// fmt.Println("==> j:", j)
			// fmt.Println("==> repo[", i, j, "]:", repo[i][j])
			// fmt.Println(repo[i][j])
			D.Data.Set(repo[i][j], (uint64(i)/p.M)*D.Info.Ne+j, uint64(i)%p.M)
		}
	}

	// Map DB elems to [-p/2; p/2]
	D.Data.Sub(p.P / 2)

	return D
}
