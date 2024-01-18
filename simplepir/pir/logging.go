package pir

import "time"
import "fmt"
import "os"
import "bufio"
import "math"
import "encoding/csv"
import "strings"
import "strconv"

// func printTime(start time.Time) time.Duration {
// 	elapsed := time.Since(start)
// 	fmt.Printf("\tElapsed: %s\n", elapsed)
// 	return elapsed
// }

func printTime(start time.Time) string {
	elapsed := time.Since(start)
	elapsedMilliseconds := elapsed.Milliseconds()
	elapsedString := fmt.Sprintf("%d ms", elapsedMilliseconds)

	fmt.Printf("\tElapsed: %s\n", elapsedString)
	return elapsedString
}

func printRate(p Params, time string, batch_sz int) float64 {
	parts := strings.Split(time, " ")
	ms,_ := strconv.ParseFloat(parts[0], 64)
	seconds := ms/1000
	RATE = math.Log2(float64((p.P))) * float64(p.L*p.M) * float64(batch_sz) /
		float64(8*1024*1024*seconds)
	fmt.Printf("\tRate: %f MB/s\n", RATE)
	return RATE
}

func writeToCSV() {
	// filePath := "../../results.csv"
	// filePath := "../../doublepir_results.csv"
	// filePath := "../../concurrent_results.csv"
	// filePath := "../../tor_results.csv"
	filePath := "../../results/results.csv"

	if (FILE_NAME != "") {
		filePath = "../../results/" + FILE_NAME + ".csv"
	}
	// fmt.Println("==> Saving to:", filePath)

	// Check if the file exists
	_, err := os.Stat(filePath)
	fileExists := !os.IsNotExist(err)

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening CSV file:", err)
		return
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	if !fileExists {
		columnNames := []string{
			"REPO_SIZE", "ROWS", "COLS", "CHUNKS", "ORIGINAL_REPO_SIZE", "PADDED_REPO_SIZE", "ITERATION", "FILE_SIZE", "SERVER_DB_SIZE", "SERVER_DB_SETUP_TIME",
			"SERVER_DB_SETUP_OFFLINE_DOWNLOAD", "CLIENT_BUILDING_QUERY", "CLIENT_BUILDING_QUERY_ONLINE_UPLOAD",
			"SERVER_ANSWER_QUERY_TIME", "SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD", "CLIENT_RECONSTRUCTION_TIME","SICES_SAME", "RATE", "NUM_DB", OPTIONAL_FIELD, OPTIONAL_FIELD_2,
		}
		writer.Write(columnNames)
	}
	
	row := []string{
		fmt.Sprintf("%v", REPO_SIZE),
		fmt.Sprintf("%v", ROWS),
		fmt.Sprintf("%v", COLS),
		fmt.Sprintf("%v", CHUNKS),
		fmt.Sprintf("%v", ORIGINAL_REPO_SIZE),
		fmt.Sprintf("%v", PADDED_REPO_SIZE),
		fmt.Sprintf("%v", ITERATION),
		fmt.Sprintf("%v", FILE_SIZE),
		fmt.Sprintf("%v", SERVER_DB_SIZE),
		fmt.Sprintf("%v", SERVER_DB_SETUP_TIME),
		fmt.Sprintf("%v", SERVER_DB_SETUP_OFFLINE_DOWNLOAD),
		fmt.Sprintf("%v", CLIENT_BUILDING_QUERY),
		fmt.Sprintf("%v", CLIENT_BUILDING_QUERY_ONLINE_UPLOAD),
		fmt.Sprintf("%v", SERVER_ANSWER_QUERY_TIME),
		fmt.Sprintf("%v", SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD),
		fmt.Sprintf("%v", CLIENT_RECONSTRUCTION_TIME),
		fmt.Sprintf("%v", SICES_SAME),
		fmt.Sprintf("%v", RATE),
		fmt.Sprintf("%v", NUM_DB),
		fmt.Sprintf("%v", OPTIONAL_FIELD_VALUE),
		fmt.Sprintf("%v", OPTIONAL_FIELD_2_VALUE),
	}

	writer.Write(row)
}

	writer.Write(row)
}

// func printRate(p Params, elapsed time.Duration, batch_sz int) float64 {
// 	rate := math.Log2(float64((p.P))) * float64(p.L*p.M) * float64(batch_sz) /
// 		float64(8*1024*1024*elapsed.Seconds())
// 	fmt.Printf("\tRate: %f MB/s\n", rate)
// 	return rate
// }

func clearFile(filename string) {
	f, err := os.OpenFile(filename, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if _, err = f.WriteString("log(n) log(l) log(m) log(q) rate(MB/s) BW(KB)\n"); err != nil {
		panic(err)
	}
}

func writeToFile(p Params, rate, bw float64, filename string) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	fmt.Fprintf(w,
		"%d,%d,%d,%d,%f,%f\n",
		int(math.Log2(float64(p.N))),
		int(math.Log2(float64(p.L))),
		int(math.Log2(float64(p.M))),
		p.Logq,
		rate,
		bw)
	w.Flush()
}
