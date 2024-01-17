package pir

import (
	"fmt"
	"testing"
	// "io"
	"net"
	// "os"
	// "path/filepath"
	// "strconv"
	// "strings"
	"encoding/gob"
)

const LOGQ = uint64(32)
const SEC_PARAM = uint64(1 << 10)
var ROWS = 999
var COLS = 999
var ITERATION = 999
var SERVER_DB_SIZE = ""
var FILE_SIZE = ""
var SERVER_DB_SETUP_TIME = ""
var SERVER_DB_DIMENSION = ""
var SERVER_DB_SETUP_OFFLINE_DOWNLOAD = ""
var CLIENT_BUILDING_QUERY = ""
var CLIENT_BUILDING_QUERY_ONLINE_UPLOAD = ""
var SERVER_ANSWER_QUERY_TIME = ""
var SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD = ""
var CLIENT_RECONSTRUCTION_TIME = ""
var RESULTS [][]uint64

type DataContainer struct {
	DBInfo 			DBinfo
	SharedState 	State
	P				Params
	OfflineDownload Msg
	BatchSize		uint64
	MetaData		[][]uint64
}

type QueryDataContainer struct {
	Queries 		[]MsgSlice
	ClientStates [][]State
}

type AnswerDataContainer struct {
	Answers 		[]Msg
}

////////////////////////////////////////////

func receiveData(conn net.Conn, target interface{}) (error) {
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func sendData(conn net.Conn, data interface{}) error {
	encoder := gob.NewEncoder(conn)
	if err := encoder.Encode(data); err != nil {
		return err
	}
	return nil
}

func client() {
	conn, err := net.Dial("tcp", "localhost:12345")
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()


	// // Ask user for folder choice
	// var folderChoice int
	// fmt.Print("Enter the number of the folder to receive: ")
	// _, err = fmt.Scanln(&folderChoice)
	// if err != nil {
	// 	fmt.Println("Error reading input:", err)
	// 	return
	// }

	// // Send folder choice to server
	// _, err = conn.Write([]byte(strconv.Itoa(folderChoice) + "\n"))
	// if err != nil {
	// 	fmt.Println("Error sending folder choice:", err)
	// 	return
	// }


	receivedData := DataContainer{}
	if err := receiveData(conn, &receivedData); err != nil {
		fmt.Println("Error receiving State data:", err.Error())
		return
	}

	fmt.Println("receivedData:", receivedData.MetaData)
	pir := SimplePIR{}

	i := []uint64{0}
	fmt.Println(receivedData.MetaData[0])

	num_chunks := int(receivedData.MetaData[0][1])
	chunks_start := int(receivedData.MetaData[0][0])
	chunks_end := chunks_start + num_chunks

	queries := make([]MsgSlice, 0, num_chunks)
	client_states := make([][]State, 0, num_chunks)

	fmt.Println("\n(CLIENT) Building query...")
	for j := chunks_start; j < chunks_end; j++ {
		i = []uint64{uint64(j)}
		query, client_state := BuildingQuery(receivedData.DBInfo, i, receivedData.SharedState, receivedData.BatchSize, receivedData.P, &pir)
		queries = append(queries, query)
		client_states = append(client_states, client_state)
	}

	query_container := QueryDataContainer{
		Queries: 		queries,
		ClientStates: 	client_states,
	}

	err = sendData(conn, query_container)
	if err != nil {
		fmt.Println("Error sending data:", err.Error())
		return
	}

	receivedAnswers := AnswerDataContainer{}
	if err := receiveData(conn, &receivedAnswers); err != nil {
		fmt.Println("Error receiving AnswerData:", err.Error())
		return
	}

	fmt.Println("answer:", receivedAnswers)

	results := make([]uint64, 0, num_chunks)

	fmt.Println("(CLIENT) Reconstructing...")
	for j := chunks_start; j < chunks_end; j++ {
		i = []uint64{uint64(j)}
		result := ReconstructingAnswer(i, receivedData.BatchSize, receivedData.OfflineDownload, queries[j], receivedAnswers.Answers[j], receivedData.SharedState, client_states[j], receivedData.DBInfo, receivedData.P, &pir, receivedData.MetaData[j])
		results = append(results, result)
	}


	reconstructedBytes := reconstructBytes(RESULTS, num_chunks)

	convertBytesToRepo(reconstructedBytes, 5)






	// // Receive folder name from server
	// n, err = conn.Read(buffer)
	// if err != nil {
	// 	fmt.Println("Error receiving folder name:", err)
	// 	return
	// }
	// folderName := strings.TrimSpace(string(buffer[:n]))

	// // Create a new folder with the received folder name
	// err = os.MkdirAll(folderName, os.ModePerm)
	// if err != nil {
	// 	fmt.Println("Error creating folder:", err)
	// 	return
	// }

	// // Create a new file to receive the folder contents
	// file, err := os.Create(filepath.Join(folderName, folderName))
	// if err != nil {
	// 	fmt.Println("Error creating file:", err)
	// 	return
	// }
	// defer file.Close()

	// // Receive and save the folder contents from server
	// _, err = io.Copy(file, conn)
	// if err != nil {
	// 	fmt.Println("Error receiving folder contents:", err)
	// 	return
	// }

	// fmt.Println("Folder received:", folderName)
}


////////////////////////////////////////////

func TestServer(t *testing.T) {

	client()
}
