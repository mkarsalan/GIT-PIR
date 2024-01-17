package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:12345")
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	// Receive and display folder list
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error receiving folder list:", err)
		return
	}
	fmt.Println(string(buffer[:n]))

	// Ask user for folder choice
	var folderChoice int
	fmt.Print("Enter the number of the folder to receive: ")
	_, err = fmt.Scanln(&folderChoice)
	if err != nil {
		fmt.Println("Error reading input:", err)
		return
	}

	// Send folder choice to server
	_, err = conn.Write([]byte(strconv.Itoa(folderChoice) + "\n"))
	if err != nil {
		fmt.Println("Error sending folder choice:", err)
		return
	}

	// Receive folder name from server
	n, err = conn.Read(buffer)
	if err != nil {
		fmt.Println("Error receiving folder name:", err)
		return
	}
	folderName := strings.TrimSpace(string(buffer[:n]))

	// Create a new folder with the received folder name
	err = os.MkdirAll(folderName, os.ModePerm)
	if err != nil {
		fmt.Println("Error creating folder:", err)
		return
	}

	// Create a new file to receive the folder contents
	file, err := os.Create(filepath.Join(folderName, folderName))
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	// Receive and save the folder contents from server
	_, err = io.Copy(file, conn)
	if err != nil {
		fmt.Println("Error receiving folder contents:", err)
		return
	}

	fmt.Println("Folder received:", folderName)
}