package pir

import (
	"fmt"
	// "os"
	"runtime"
	// "runtime/debug"
	// "runtime/pprof"
	"time"
	// "github.com/mohae/deepcopy"
	// "math"
	"strconv"
	"sync"
	// "reflect"
	// "strings"
	// "bufio"
	"net"
	"os/exec"
	"encoding/gob"
)

func BuildingQueryConcurrently_(DBinfo DBinfo, i []uint64, shared_state State, batch_sz uint64, p Params, pi PIR, index int, queryChan chan QueryChannel, clientWG *sync.WaitGroup) {
	defer clientWG.Done()
	var client_state []State
	var query MsgSlice

	for index_, _ := range i {
		index_to_query := i[index_] + uint64(index_)*batch_sz
		cs, q := pi.Query(index_to_query, shared_state, p, DBinfo)
		client_state = append(client_state, cs)
		query.Data = append(query.Data, q)
	}

	queryChan <- QueryChannel{Index: index, Query: query, States: client_state}
}

func AnsweringQueryConcurrently_(DB *Database, query MsgSlice, server_state State, shared_state State, p Params, pi PIR, index int, answerChan chan AnswerChannel, serverWG *sync.WaitGroup) {
	defer serverWG.Done()
	answer := pi.Answer(DB, query, server_state, shared_state, p)
	pi.Reset(DB, p)

	answerChan <- AnswerChannel{Index: index, Answer: answer}
}

func ReconstructingAnswerConcurrently_(i []uint64, batch_sz uint64, offline_download Msg, query MsgSlice, answer Msg, shared_state State, client_state []State, DBinfo DBinfo, p Params, pi PIR, metadata []uint64, db_index int,reconstructingAnswerChan chan ReconstructingAnswerChannel, wg *sync.WaitGroup) {
	defer wg.Done()
	var vals []uint64

	for index, _ := range i {
		index_to_query := i[index] + uint64(index)*batch_sz
		vals = pi.RecoverRepository(index_to_query, uint64(index), offline_download, query.Data[index], answer, shared_state, client_state[index], p, DBinfo, metadata)
	}

	reconstructingAnswerChan <- ReconstructingAnswerChannel{Index: db_index, Answer: vals}
}


/////////////////////////////////////

type Message struct {
	Text string
}

type BuildingQueryMetaData struct {
    LookupTable []LookupTable
    Index []uint64
    SharedState []State
    BatchSize uint64
    Params Params
    DBInfo []DBinfo
    OfflineDownload []Msg
    NumDB int
    Rows int
    Cols int
}

type AnsweringQueryMetaData struct {
    Queries []MsgSlice
}

type AnswerReconstructionMetaData struct {
    Answers []Msg
}


func handleConnection(conn net.Conn, buildingQueryMetaData BuildingQueryMetaData, DB []*Database, server_state []State, shared_state []State, p Params, pi PIR, index uint64) {
	defer conn.Close()

	// messageToSend := Message{Text: "Hello, Client!"}

	// Send the Message to the client
	encoder := gob.NewEncoder(conn)
	if err := encoder.Encode(buildingQueryMetaData); err != nil {
		fmt.Println("Error encoding message:", err)
		return
	}

	var receivedAnsweringQueryMetaData AnsweringQueryMetaData
    decoder := gob.NewDecoder(conn)
    if err := decoder.Decode(&receivedAnsweringQueryMetaData); err != nil {
        fmt.Println("Error decoding AnsweringQueryMetaData from client:", err)
        return
    }

    // SERVER
    numDBs := len(DB)
	start_answering := time.Now()
	comm := float64(0)
	var serverWG sync.WaitGroup
	answerChan := make(chan AnswerChannel, numDBs)
	answers := make([]Msg, numDBs)
	fmt.Println("(SERVER) Answering query...")
	for j := 0; j < numDBs; j++ {
		serverWG.Add(1)
		go AnsweringQueryConcurrently_(DB[j], receivedAnsweringQueryMetaData.Queries[j], server_state[j], shared_state[j], p, pi, j, answerChan, &serverWG)
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

	answerReconstructionMetaData := AnswerReconstructionMetaData{
	    Answers: 	answers,
	}

	if err := encoder.Encode(answerReconstructionMetaData); err != nil {
		fmt.Println("Error encoding message:", err)
		return
	}

	// // Read the message from the client
	// message, err := bufio.NewReader(conn).ReadString('\n')
	// if err != nil {
	// 	fmt.Println("Error reading from client:", err)
	// 	return
	// }

	// // Print the received message
	// fmt.Printf("Server received: %s", message)
}

func startServer(buildingQueryMetaData BuildingQueryMetaData, DB []*Database, server_state []State, shared_state []State, p Params, pi PIR, index uint64) {
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Server is listening on localhost:8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			break
		}

		// Handle the connection in a separate goroutine
		go handleConnection(conn, buildingQueryMetaData, DB, server_state, shared_state, p, pi, index)
	}
}



///////////////////////////////////// 


func GITPIR_Multiserver(DB []*Database, i []uint64, shared_state []State, server_state []State, batch_sz uint64, p Params, pi PIR, offline_download []Msg, lookupTable []LookupTable) []uint64 {
	numDBs := len(DB)

	///////////////////////////////////// 
	DBinfo := make([]DBinfo, numDBs)
	for j := 0; j < numDBs; j++ {
		DBinfo[j] = DB[j].Info
	}

	fmt.Println("\nServer Established..\nInitializing Client\nExectuing Clone Request:")

	buildingQueryMetaData := BuildingQueryMetaData{
	    LookupTable:		lookupTable,
	    Index: 				i,
	    SharedState: 		shared_state,
	    BatchSize: 			batch_sz,
	    Params: 			p,
	    DBInfo:				DBinfo,
	    OfflineDownload:	offline_download,
	    NumDB:				numDBs,
	    Rows: 				ROWS,
	    Cols:				COLS,
	}

	go startServer(buildingQueryMetaData, DB, server_state, shared_state, p, pi, i[0])

	fmt.Println("##############################")

	time.Sleep(1000000000)
	exec.Command("go", "test", "-run", "Client")
	
	return i

	///////////////////////////////////// 


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
		go BuildingQueryConcurrently_(DB[j].Info, i, shared_state[j], batch_sz, p, pi, j, queryChan, &clientWG)
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
		go AnsweringQueryConcurrently_(DB[j], queries[j], server_state[j], shared_state[j], p, pi, j, answerChan, &serverWG)
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
		go ReconstructingAnswerConcurrently_(i, batch_sz, offline_download[j], queries[j], answers[j], shared_state[j], client_states[j], DB[j].Info, p, pi, []uint64{0}, j, reconstructingAnswerChan, &wg)
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

	writeToCSV()

	return ans

}