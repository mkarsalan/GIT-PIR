package pir

import (
	"fmt"
	"time"
	"github.com/mohae/deepcopy"
	// "strconv"
	"sync"
"bytes"
	"encoding/gob"
)

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

	Reset(DB *Database, p Params) // reset DB to its correct state, if modified during execution
}


func RunGIT_PIR(pi PIR, DB *Database, p Params, i []uint64, metadata [][]uint64) (string) {
	fmt.Printf("Executing %s\n", pi.Name())
	num_queries := uint64(len(i))

	if DB.Data.Rows/num_queries < DB.Info.Ne {
		panic("Too many queries to handle!")
	}

	batch_sz := DB.Data.Rows / (DB.Info.Ne * num_queries) * DB.Data.Cols
	shared_state := pi.Init(DB.Info, p)

	fmt.Println("\n(SERVER) DB Setup...")
	// start := time.Now()
	server_state, offline_download := pi.Setup(DB, shared_state, p)	
	DB.Data.Dim()

	comm := float64(offline_download.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	fmt.Println("p.Logq:", p.Logq)

	// time := printTime(start)
	fmt.Printf("\t\tOffline download: %f KB\n", comm)

	// SERVER_DB_SETUP_TIME = time
	// SERVER_DB_SETUP_OFFLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"

	// ClientServerPIRConcurrently(DB, i, shared_state, server_state, batch_sz, p, pi, offline_download, metadata)

	ClientServerPIR(DB, i, shared_state, server_state, batch_sz, p, pi, offline_download, metadata)

	writeToCSV()
	return ""
}

func ClientServerPIR(DB *Database, i []uint64, shared_state State, server_state State, batch_sz uint64, p Params, pi PIR, offline_download Msg, metadata [][]uint64) {
	
	fmt.Println("\n\n==> i:", i) //
	fmt.Println("\n==> DB.Info:", DB.Info)
	fmt.Println("\n==> shared_state:", shared_state)
	fmt.Println("\n==> batch_sz:", batch_sz) //
	fmt.Println("\n==> p:", p)
	fmt.Println("\n==> pi:", pi)

	// Serialize the data to bytes
	dataBytes, err := structToBytes(shared_state)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
	}

	// Deserialize and print the data on the server for demonstration
	receivedData := State{}
	err = bytesToStruct(dataBytes, &receivedData)
	if err != nil {
		fmt.Println("Error decoding and receiving data:", err)
	}

	shared_state = receivedData

	//////////////////////////////
	// Serialize the data to bytes
	dataBytes_DB_Info, err := structToBytes(DB.Info)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
	}

	// Deserialize and print the data on the server for demonstration
	receivedData_DB_Info := DBinfo{}
	err = bytesToStruct(dataBytes_DB_Info, &receivedData_DB_Info)
	if err != nil {
		fmt.Println("Error decoding and receiving data:", err)
	}

	DB.Info = receivedData_DB_Info

	//////////////////////////////
	// Serialize the data to bytes
	dataBytes_p, err := structToBytes(p)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
	}

	// Deserialize and print the data on the server for demonstration
	receivedData_p := Params{}
	err = bytesToStruct(dataBytes_p, &receivedData_p)
	if err != nil {
		fmt.Println("Error decoding and receiving data:", err)
	}

	p = receivedData_p

	// //////////////////////////////
	// // Serialize the data to bytes
	// dataBytes_pi, err := structToBytes(pi)
	// if err != nil {
	// 	fmt.Println("Error encoding and sending data:", err)
	// }

	// // Deserialize and print the data on the server for demonstration
	// receivedData_pi := PIR{}
	// err = bytesToStruct(dataBytes_pi, &receivedData_pi)
	// if err != nil {
	// 	fmt.Println("Error decoding and receiving data:", err)
	// }

	// pi = receivedData_pi

	// pi := pi.Init(DB.Info, p)
	// pir_params := Params{
	// 		N:    p.N,
	// 		Logq: p.Logq,
	// 		L:    p.L,
	// 		M:    p.M,
	// 	}

	// pi = pi.Init(DB.Info, pir_params)




	// CLIENT
	query, client_state := BuildingQuery(DB.Info, i, shared_state, batch_sz, p, pi)
	
	// SERVER
	answer := AnsweringQuery(DB, query, server_state, shared_state, p, pi)
	
	// CLIENT
	result := ReconstructingAnswer(i, batch_sz, offline_download, query, answer, shared_state, client_state, DB.Info, p, pi, metadata[0])
	
	fmt.Printf("\n\n*************\n\nREQUESTED REPO: %v\n\n*************\n\n", result)
	return
}

func ReconstructingAnswer(i []uint64, batch_sz uint64, offline_download Msg, query MsgSlice, answer Msg, shared_state State, client_state []State, DBinfo DBinfo, p Params, pi PIR, metadata []uint64) uint64 {
	// fmt.Println("(CLIENT) Reconstructing...")
	// start := time.Now()
	result := uint64(0)
	for index, _ := range i {
		index_to_query := i[index] + uint64(index)*batch_sz
		val := pi.RecoverFile(index_to_query, uint64(index), offline_download, query.Data[index], answer, shared_state, client_state[index], p, DBinfo, metadata)
		result = val
	}
	// time := printTime(start)

	// CLIENT_RECONSTRUCTION_TIME = time

	return result
}

func AnsweringQuery(DB *Database, query MsgSlice, server_state State, shared_state State, p Params, pi PIR) Msg {
	fmt.Println("(SERVER) Answering query...")
	// start := time.Now()
	answer := pi.Answer(DB, query, server_state, shared_state, p)
	// time := printTime(start)
	comm := float64(answer.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	fmt.Printf("\t\tOnline download: %f KB\n", comm)
	pi.Reset(DB, p)

	// SERVER_ANSWER_QUERY_TIME = time
	// SERVER_ANSWER_QUERY_ONLINE_DOWNLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"

	return answer
}

func BuildingQuery(DBinfo DBinfo, i []uint64, shared_state State, batch_sz uint64, p Params, pi PIR) (MsgSlice, []State) {
	// fmt.Println("\n(CLIENT) Building query...")
	var client_state []State
	var query MsgSlice
	// start := time.Now()
	for index, _ := range i {
		index_to_query := i[index] + uint64(index)*batch_sz
		cs, q := pi.Query(index_to_query, shared_state, p, DBinfo)
		client_state = append(client_state, cs)
		query.Data = append(query.Data, q)
	}
	// time := printTime(start)
	// comm := float64(query.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	// fmt.Printf("\t\tOnline upload: %f KB\n", comm)

	// CLIENT_BUILDING_QUERY = time
	// CLIENT_BUILDING_QUERY_ONLINE_UPLOAD = strconv.FormatFloat(comm, 'f', -1, 64) + " KB"
	
	return query, client_state
}

///////////////////////////////////////////////

func ClientServerPIRConcurrently(DB *Database, i []uint64, shared_state State, server_state State, batch_sz uint64, p Params, pi PIR, offline_download Msg, metadata [][]uint64) {
	chunks := int(metadata[0][1])
	deepCopyDB := deepcopy.Copy(DB).(*Database)

	// CLIENT
	queries := make([]MsgSlice, 0, chunks)
	client_states := make([][]State, 0, chunks)
	for j := 0; j < chunks; j++ {
		query, client_state := BuildingQuery(DB.Info, []uint64{uint64(j)}, shared_state, batch_sz, p, pi)
		queries = append(queries, query)
		client_states = append(client_states, client_state)
	}

	// SERVER
	start_answering := time.Now()
	var serverWG sync.WaitGroup
	answerChan := make(chan Msg, chunks)
	answers := make([]Msg, 0, chunks)
	fmt.Println("(SERVER) Answering query...")
	for j := 0; j < chunks; j++ {
		serverWG.Add(1)
		DB = deepcopy.Copy(deepCopyDB).(*Database)
		go AnsweringQueryConcurrently(DB, queries[j], server_state, shared_state, p, pi, answerChan, &serverWG)
	}
	serverWG.Wait()
	close(answerChan)
	for j := 0; j < chunks; j++ {
		x := <-answerChan
		answers = append(answers, x)
	}
	printTime(start_answering)

	// CLIENT
	start_reconstruction := time.Now()
	var wg sync.WaitGroup
	fmt.Println("(CLIENT) Reconstructing...")
	for j := 0; j < chunks; j++ {
		wg.Add(1)
		go ReconstructingAnswerConcurrently(i, batch_sz, offline_download, queries[j], answers[j], shared_state, client_states[j], DB.Info, p, pi, metadata[j], &wg)
	}
	wg.Wait()
	printTime(start_reconstruction)

	return
}

func ReconstructingAnswerConcurrently(i []uint64, batch_sz uint64, offline_download Msg, query MsgSlice, answer Msg, shared_state State, client_state []State, DBinfo DBinfo, p Params, pi PIR, metadata []uint64, wg *sync.WaitGroup) {
	defer wg.Done()
	for index, _ := range i {
		index_to_query := i[index] + uint64(index)*batch_sz
		pi.RecoverFile(index_to_query, uint64(index), offline_download, query.Data[index], answer, shared_state, client_state[index], p, DBinfo, metadata)
	}
}

func AnsweringQueryConcurrently(DB *Database, query MsgSlice, server_state State, shared_state State, p Params, pi PIR, answerChan chan Msg, serverWG *sync.WaitGroup) {
	defer serverWG.Done()
	answer := pi.Answer(DB, query, server_state, shared_state, p)
	pi.Reset(DB, p)
	answerChan <- answer
}

///////////////////////////////////////////////



// Helper function to convert a struct to bytes using gob
func structToBytes(data interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	if err := encoder.Encode(data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Helper function to convert bytes back to a struct using gob
func bytesToStruct(dataBytes []byte, data interface{}) error {
	buffer := bytes.NewBuffer(dataBytes)
	decoder := gob.NewDecoder(buffer)
	if err := decoder.Decode(data); err != nil {
		return err
	}
	return nil
}



