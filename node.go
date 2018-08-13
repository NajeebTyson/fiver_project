package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

// server msg protocols
var (
	HEARTBEAT   string = "HBT"
	DUPLICATE   string = "DUP"
	ALIVE       string = "ALV"
	NOTALIVE    string = "NAL"
	SEARCH      string = "SRH"
	ABORT       string = "ABT"
	OKABORT     string = "OBT"
	FOUND       string = "FND"
	NOTFOUND    string = "NFD"
	FREE        string = "FRI"
	NOTFREE     string = "NFR"
	EXIT        string = "EXT"
	LOADFILE    string = "LOD"
	UNLOADFILE  string = "ULD"
	MEMORYFILES string = "MMF"
)

// seperator
var SEP string = "*|*"

// free or busy status
var STATUS string = FREE

// rank of node
var Rank int

// display heartbeat messages
var displayHeartbeat = false

// data content
var DATA [][]string

// keep track of files which loaded in memory
var filesInMem []string

// display error function
func checkError(e error) {
	if e != nil {
		log.Fatal(e)
		panic(e)
	}
}

// load file with given name in directory of given node
func loadFile(filename string, id int) []string {
	filePath := fmt.Sprintf("./data/node_%d/%s", id, filename)

	//checking file size
	fileStat, e := os.Stat(filePath)
	checkError(e)
	fileSize := fileStat.Size() // file size
	log.Printf("Node [%d] opening file [%s] of size %d MB\n", id, filename, fileSize/1024/1024)

	file, err := os.Open(filePath) // open file
	checkError(err)
	defer file.Close()

	// buffer to load data in
	data := make([]byte, fileSize)
	bytesStream, e1 := file.Read(data) // read content
	checkError(e1)
	log.Printf("Node %d streamed %d MB data of file %s\n", id, bytesStream/1024/1024, filename)

	dataS := string(data) // converting data to string from []byte
	flines := make([]string, 0)
	flines = strings.Split(dataS, "\n") // splitting content by \n

	return flines
}

// search the given text in global DATA
func searchText(text string) int {
	// idx := sort.SearchStrings(data, text)
	// fmt.Println("Data length: ", len(DATA))
	idx := -1
	stopSearch := false         // stop search if this is true
	for dataIdx := range DATA { // loop over data content one by one
		if stopSearch { // check if true then stop
			break
		}
		idx = len(DATA[dataIdx])
		for i_, val := range DATA[dataIdx] { // loop over line by line
			if STATUS == NOTFREE { // status is busy so dont stop searching
				if val == text { // if text is found
					idx = i_          // storing index of text
					stopSearch = true // make the search stop now
					break
				}
			} else { // status is free so we have to stop searching
				idx = -2
				stopSearch = true
				break
			}
		}
		if idx == len(DATA[dataIdx]) { // if not found
			idx = -1
		}
	}
	return idx
}

// delete element from slice of string
func delFromSlice(s []string, i int) []string {
	return append(s[:i], s[i+1:]...)
}

// delete file content from global DATA variable
func delFromDATA(i int) {
	DATA = append(DATA[:i], DATA[i+1:]...)
}

// take action on message from server
func msgAction(msg string, sendmsgchan chan string) {
	command := msg[:3] // command or protocol
	var res string

	// server send hearbeat signal
	if command == HEARTBEAT {
		res = ALIVE // send server alive signal
	} else if command == SEARCH { // server send search query
		tokens := strings.Split(msg, SEP) // tokenize the message
		searchString := tokens[1]         // text to search
		log.Printf("Node is now searching query [%s]\n", searchString)
		clientID := tokens[2] // client id which request for search
		// fmt.Printf("Now searching: %s, len: %d\n", searchString, len(searchString))
		STATUS = NOTFREE                // status of node is busy now because node is searching now
		idx := searchText(searchString) // index after searching
		// if text is not found
		if idx == -1 {
			log.Printf("%s not found\n", searchString)
		} else if idx == -2 { // if received abort signal from server
			log.Printf("Searching for %s is terminated due to abort command\n", searchString)
		} else { // text is found
			log.Printf("%s found at index: %d\n", searchString, idx)
		}

		res = FOUND
		if idx == -1 { // if not found
			res = NOTFOUND
		}
		res += SEP + searchString
		if idx == -2 {
			return
			// res = fmt.Sprintf("%s%s%d", OKABORT, SEP, Rank)
		}
		res += SEP + clientID
	} else if command == LOADFILE { // if command is to load file from harddisk into memory
		// tokenize files which are going to be loaded
		fileToLoad := strings.Split(msg, SEP)[1]
		log.Printf("Node is now loading file into memory, [%s]\n", fileToLoad)
		for _, newFileToLoad := range strings.Split(fileToLoad, "--") { // each file to load
			filesInMem = append(filesInMem, newFileToLoad)     // append file name in variable
			DATA = append(DATA, loadFile(newFileToLoad, Rank)) // appending content in DATA
		}
		// response to node of files name in memory
		res = MEMORYFILES + SEP + strings.Join(filesInMem, "--")
	} else if command == UNLOADFILE { // if command is to unload file from memory to harddisk
		// which files to unload
		filesToUnload := strings.Split(strings.Split(msg, SEP)[1], "--")
		log.Printf("Node is now unloading files from memory, [%s]\n", filesToUnload)
		for _, fUnload := range filesToUnload {
			for mfIdx, mf := range filesInMem {
				if mf == fUnload {
					filesInMem = delFromSlice(filesInMem, mfIdx) // deleting name of file
					delFromDATA(mfIdx)                           // deleting content of file
				}
			}
		}
		res = MEMORYFILES + SEP + strings.Join(filesInMem, "--")
	} else if command == ABORT { // if command is to abort search
		log.Println("Abort command received from server")
		STATUS = FREE // make status free now
		return
	} else if command == EXIT { // if command is to exit
		log.Println("Node is now Exiting")
		os.Exit(1)
	}

	// log.Printf("In Action res: %s\n", res)
	res = res + "\n"
	sendmsgchan <- res // adding res to channel to send to server
}

// handle in and out messages of node
func handleMessages(id int, con net.Conn, sendmsgchan chan string, recvmsgchan <-chan string) {
	for {
		select {
		case msg := <-sendmsgchan:
			// log.Printf("Node %d sending data: %s\n", id, msg)
			con.Write([]byte(msg)) // writing to server

		case msg := <-recvmsgchan:
			log.Printf("Node %d received data: %s\n", id, msg)
			if len(msg) < 3 {
				log.Fatal("Something is wrong. Receving invalid commands from server\n")
				continue
			}
			// take action on server message
			go msgAction(msg, sendmsgchan)
		}
	}
}

// get list of files which are in directory of node
func getChunkList(id int) []string {
	// file names in directory
	files, err := ioutil.ReadDir(fmt.Sprintf("./data/node_%d/", id))
	if err != nil {
		log.Fatal("Failed to Read data files.\n", err)
		return nil
	}
	chunkList := make([]string, 0)
	for _, f := range files {
		if !f.IsDir() && len(f.Name()) >= 9 {
			chunkList = append(chunkList, f.Name()) // adding file to list
		}
	}
	return chunkList
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Provide rank and server address")
	}
	rank, _ := strconv.Atoi(os.Args[1])
	Rank = rank
	serverAddress := os.Args[2]

	con, err := net.Dial("tcp", serverAddress)
	checkError(err)
	defer con.Close()
	log.Println("Connection with server established.")

	// channles
	sendmsgchan := make(chan string)
	recvmsgchan := make(chan string)

	// message handler
	go handleMessages(rank, con, sendmsgchan, recvmsgchan)

	// read the chunk names of node data
	chunkFiles := getChunkList(rank)
	if len(chunkFiles) == 0 {
		panic(fmt.Sprintf("No data in the node[%d] directory\n", rank))
	}

	// read from server
	buf := bufio.NewReader(con)
	var msg []byte

	// write the rank of node to server
	con.Write([]byte(fmt.Sprintf("%d\n", rank)))
	msg, _, _ = buf.ReadLine()

	if string(msg) == DUPLICATE { // if node is already present
		fmt.Printf("Node with id [%d] is already present.\n", rank)
	}

	// write list of files in diretory to server
	con.Write([]byte(strings.Join(chunkFiles, "--") + "\n"))

	var cerr error
	for {
		msg, _, cerr = buf.ReadLine()
		if cerr != nil {
			con.Write([]byte(EXIT + SEP + strconv.Itoa(Rank) + "\n"))
		}
		recvmsgchan <- string(msg)
	}
}
