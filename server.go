package main

import (
	"bufio"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// server msg protocols
var (
	HEARTBEAT   string = "HBT"
	DUPLICATE   string = "DUP"
	OKAY        string = "OKY"
	ALIVE       string = "ALV"
	NOTALIVE    string = "NAL"
	SEARCH      string = "SRH"
	ABORT       string = "ABT"
	FOUND       string = "FND"
	NOTFOUND    string = "NFD"
	EXIT        string = "EXT"
	OKABORT     string = "OBT"
	LOADFILE    string = "LOD"
	UNLOADFILE  string = "ULD"
	MEMORYFILES string = "MMF"
)

// seperator
var SEP string = "*|*"

// display heartbeat messages
var displayHeartbeat = false

// node structure
type Node struct {
	id          int         // node id
	con         net.Conn    // node connection pointer
	ch          chan string // node channel to send and recieve messages
	hbtchan     chan string // node heartbeat channel
	chunkFiles  []string    // node list of files in node directory
	memoryFiles []string    // node list of files in node memory
}

type Client struct {
	con         net.Conn    // client connection pointer
	ch          chan string // client channel
	recvch      chan string // client recveive channel
	id          string      // client id
	rescounter  int         // client search result counter
	searchquery []byte      // client search query
}

// node method to read from node connection and write to global message channel
func (node Node) ReadFromNode(ch chan<- string, noderemchan chan<- Node) {
	bufc := bufio.NewReader(node.con)
	for {
		line, _, err := bufc.ReadLine()
		if err != nil { // if node is node connected or problem in connection
			noderemchan <- node // remove node from Nodes
			break
		}
		ch <- fmt.Sprintf("%d%s%s", node.id, SEP, string(line))
	}
}

// node method to read from node channel and write to node connection
func (node Node) writeToNodeChan(ch <-chan string) {
	for msg := range ch {
		// fmt.Printf("Sending to node: %s\n", msg)
		node.con.Write([]byte(msg + "\n"))
	}
}

// node method to handle hearbeat of node
func (node Node) handleHeartbeat(sec int, noderemchan chan<- Node) {
	ticker := time.NewTicker(time.Second * 5) // ticker to send heartbeat every n second
	func() {
		for _ = range ticker.C { // if ticker tick then send heartbeat
			node.con.Write([]byte(HEARTBEAT + "\n")) // sending node heartbeat signal
			timer := time.NewTimer(time.Second * 2)  // timer of n second for the node response of heartbeat
			checkRes := false                        // check for node response if this is true
			gotResponse := 0
			go func(timmer *time.Timer) { // waiting for ending of response time
				<-timmer.C      // block till response time not complete
				checkRes = true // you can check for response now
			}(timer)
			for {
				if gotResponse != 0 { // if got any response break
					break
				}
				select {
				case hbREs := <-node.hbtchan: // node send response to hearbeat signal
					if hbREs == ALIVE { // node is alive
						gotResponse = 1
					} else { // node is not alive
						gotResponse = -1
					}
					break
				default:
					if checkRes { // node is not alive
						gotResponse = -1
						break
					}
				}
			}
			if displayHeartbeat {
				log.Printf("Node [%d] htb response: %d\n", node.id, gotResponse)
			}
			if gotResponse == -1 { // node is not alive so delete node from Nodes
				node.ch <- EXIT // sending node to EXIT on safe hand
				noderemchan <- node
				timer.Stop()  // stoping timer
				ticker.Stop() // stoping ticker
			}
		}
	}()
}

// read from client channel and write to client connection
func (client Client) writeToClientChan(ch <-chan string) {
	for msg := range ch { // read from channel
		// fmt.Printf("Sending to cient: %s\n", msg)
		client.con.Write([]byte(msg + "\n")) // write to connection
		if msg == EXIT {                     // if client exit
			delete(Clients, client.id) // delete client
		}
	}
}

// print node
func (node Node) print() {
	fmt.Printf("{Node id: %d, files, %s, memory files: %s}\n", node.id, node.chunkFiles, node.memoryFiles)
}

// to keep track of nodes
var Nodes = make(map[int]Node)

// to keep track of clients
var Clients = make(map[string]Client)

// to take actions on messages from node
func NodeMsgAction(msg string, nodeaddchan <-chan Node, noderemchan chan Node) {
	// fmt.Printf("Message in NodeAction is %s\n", msg)
	tokens := strings.Split(msg, SEP)  // split message
	command := tokens[1][0:3]          // protocol command
	node, _ := strconv.Atoi(tokens[0]) // message sender id
	switch command {

	case FOUND: // if search query is found by node
		log.Printf("%s found by node [%d]\n", tokens[2], node)
		for _, anode := range Nodes { // loop over every other node to send abort signal
			if anode.id != node { // if the node is not itself
				anode.ch <- ABORT // sending node to abort search
			}
		}
		reqClientId := tokens[3] // id of client which request for search
		aclient := Clients[reqClientId]
		Clients[reqClientId] = aclient
		aclient.recvch <- fmt.Sprintf("%s%s%d", FOUND, SEP, node) // send client found information
		aclient.recvch <- EXIT                                    // send client to exit

	case NOTFOUND: // if search query is note found by node
		log.Printf("%s not found by node [%d]\n", tokens[2], node)
		reqClientId := tokens[3] //  id of client which request for search
		aclient := Clients[reqClientId]
		aclient.rescounter += 1 // increment counter that a node does not found
		// log.Println("Counter is incremented", aclient.rescounter, string(aclient.searchquery))
		Clients[reqClientId] = aclient
		if len(Nodes) == aclient.rescounter { // if query is not found by any node
			// fmt.Printf("Sending  NOT FOUND to client\n")
			aclient.recvch <- fmt.Sprintf("%s%s0", NOTFOUND, SEP) // sending client not found signal
			aclient.recvch <- EXIT                                // sending client to exit
		}

	case MEMORYFILES: // if node send its files list in memory
		log.Printf("Nodes %d memory files: %s\n", node, tokens)
		// Nodes[node].memoryFiles = strings.Split(tokens[1], "--")
		mnode := Nodes[node]
		mnode.memoryFiles = strings.Split(tokens[2], "--") // convert those files string into list
		log.Printf("Node [%d] memory files: %s\n", node, mnode.memoryFiles)
		Nodes[node] = mnode

	case ALIVE: // if node send alive signal
		Nodes[node].hbtchan <- ALIVE // passing alive signal to that node heartbeat channel

	case NOTALIVE: // if node send not alive signal
		Nodes[node].hbtchan <- NOTALIVE // passing signal to node heartbeat channel

	case OKABORT: // if node send response of abort signal
		log.Printf("OK ABORT signal from node: %d\n", node)

	case EXIT: // if node is exiting
		nodeId, _ := strconv.Atoi(tokens[2])
		noderemchan <- Nodes[nodeId] // removing node from Nodes

	default:
		log.Println("Default msg: %s\n", msg)
	}
}

// handle file after node killed
func handleKilledNode(node Node) {
	fileHandled := false // true if file is handled
	// looking for files which are in current node memory
	for _, f := range node.memoryFiles {
		// looking for above file in all nodes
		for id, anode := range Nodes {
			if fileHandled {
				break
			}
			if node.id != id { // don't look if the node is same node
				// looking over files which are in nodes dir
				for _, chunkF := range anode.chunkFiles {
					if fileHandled {
						break
					}
					if chunkF == f { // if same file is in present in another node
						memFCounter := 0
						// if that file is already in another node memory or not
						for _, memF := range anode.memoryFiles {
							if memF != f {
								memFCounter += 1
							}
						}
						// it means file is not in memory but dir
						// so load file
						if memFCounter == len(anode.memoryFiles) {
							anode.ch <- LOADFILE + SEP + f
							fileHandled = true
							break
						}
					}
				}
			}
		}
	}
	delete(Nodes, node.id) // delete node
}

// handle messages from node
func handleNodeMessages(nodemsgchan <-chan string, nodeaddchan <-chan Node, noderemchan chan Node) {
	for {
		select {
		case msg := <-nodemsgchan: // read from global message channel
			// log.Printf("Node > %s\n", msg)
			go NodeMsgAction(msg, nodeaddchan, noderemchan) // take action on message

		case node := <-nodeaddchan: // read node from add node channel
			log.Printf("New node %d is connected, address: %s\n", node.id, node.con.RemoteAddr().String())
			Nodes[node.id] = node // new node is added

		case node := <-noderemchan: // read node from remove node channel
			log.Printf("Node %d is disconnected, address %s\n", node.id, node.con.RemoteAddr().String())
			handleKilledNode(node) // manage node data and kill node
		}
	}
}

// return true if node is present with given id
func isNodePresent(id int) bool {
	for ID := range Nodes {
		if ID == id {
			return true
		}
	}
	return false
}

// find unions in the list
func getUnions(s1 []string, s2 []string) []string {
	var unions []string
	for _, a1 := range s1 {
		for _, a2 := range s2 {
			if a1 == a2 {
				unions = append(unions, a1)
			}
		}
	}
	return unions
}

// files which are not loaded yet
func getAbsentFiles(newFiles []string) []string {
	absentFiles := make(map[string]int) // to store absent files in map
	for _, nf := range newFiles {       // over each new file
		isPresent := false
		for ID := range Nodes { // looking in each node
			if isPresent { // if present don't go further
				break
			}
			// in each node loop over each file in memory
			for _, pf := range Nodes[ID].memoryFiles {
				if nf == pf { // if new file is present in memory
					isPresent = true
					break
				}
			}
		}
		if !isPresent { // new file is not present file
			absentFiles[nf] = 0
		}
	}
	var absFiles []string
	for k := range absentFiles { // make array of absent files
		absFiles = append(absFiles, k) // appending each absent file
	}
	fmt.Println("List of new files which are not in memory: ", absFiles)
	return absFiles
}

// handle new node files
func manageNewNodeFiles(node *Node, buf *bufio.Reader) {
	// get absent file of node mean these are not loaded in any other node
	absentFiles := getAbsentFiles(node.chunkFiles)
	if len(absentFiles) > 0 { // if there is atleast file which is not loaded anywhere
		// tell the node to load this file immediatly
		node.con.Write([]byte(fmt.Sprintf("%s%s%s\n", LOADFILE, SEP, strings.Join(absentFiles, "--"))))
		msg, _, _ := buf.ReadLine()
		command := string(msg)
		// fmt.Println("In Manage: ", command)
		if command[:3] == MEMORYFILES { // if node send command after loaded file
			tokens := strings.Split(command, SEP)
			node.memoryFiles = strings.Split(tokens[1], "--") // save this file in node memory file variable
		}
	} else { // if every file of this node is already loaded then manage the load on other nodes
		for nodeID := range Nodes { // looking over each node
			// take the load of othr node if its memory file is double than yours
			if len(node.memoryFiles) >= len(Nodes[nodeID].memoryFiles)/2 {
				continue // if less then no need to take the load of that node
			}
			if len(Nodes[nodeID].chunkFiles) > 1 {
				// take the union of files which are in your directory and that node memory
				unionFiles := getUnions(node.chunkFiles, Nodes[nodeID].memoryFiles)
				noFiletoShift := 1 // how many files to shift to this node memory
				if len(unionFiles) > 1 {
					noFiletoShift = len(Nodes[nodeID].memoryFiles) / len(unionFiles)
				}
				// telling that node to unload these files
				Nodes[nodeID].ch <- UNLOADFILE + SEP + strings.Join(unionFiles[:noFiletoShift], "--")
				// telling this node to load these files
				node.con.Write([]byte(LOADFILE + SEP + strings.Join(unionFiles[:noFiletoShift], "--") + "\n"))
				msg, _, _ := buf.ReadLine()
				command := string(msg)
				// fmt.Println("In Manage: ", command)
				if command[:3] == MEMORYFILES { // rececing memory files list from node
					tokens := strings.Split(command, SEP) // tokenizing message
					node.memoryFiles = strings.Split(tokens[1], "--")
				}
			}
		}
	}
}

// handle connection from nodes
func handleNodeConnection(c net.Conn, nodemsgchan chan<- string, nodeaddchan chan<- Node, noderemchan chan<- Node) {
	defer c.Close()

	// new node
	var node Node
	node.con = c
	node.ch = make(chan string)
	node.hbtchan = make(chan string)

	// reading slave rank
	buf := bufio.NewReader(c)
	msg, _, _ := buf.ReadLine()
	node.id, _ = strconv.Atoi(string(msg))
	// check if node is already present with this id
	if isNodePresent(node.id) {
		c.Write([]byte(DUPLICATE + "\n")) // sending node that node with this id is already present
		c.Close()                         // close the node connection
		return
	}
	c.Write([]byte(OKAY + "\n")) // signal node everything is ok
	// reading slave chunks
	msg, _, _ = buf.ReadLine()                         // reading directory files of node
	node.chunkFiles = strings.Split(string(msg), "--") // spliting directory files

	// deal with new node files
	manageNewNodeFiles(&node, buf)

	// adding node to channel
	nodeaddchan <- node
	// time in seconds after whhich server send heartbeat signal to node
	heartbeatCheckInterval := 5
	// handle node hearbeat
	go node.handleHeartbeat(heartbeatCheckInterval, noderemchan)
	// handle message from node connection
	go node.ReadFromNode(nodemsgchan, noderemchan)

	defer func() {
		log.Printf("Node %d is disconnected. Connection closed with %s\n", node.id, node.con.RemoteAddr().String())
		noderemchan <- node
	}()
	node.writeToNodeChan(node.ch)
}

// accept new node connection
func acceptNodeConnections(ln net.Listener, nodemsgchan chan<- string, nodeaddchan chan<- Node, noderemchan chan<- Node) {
	for {
		conn, err := ln.Accept() // accept new node
		if err != nil {
			log.Println(err)
			continue
		}
		// handle new node
		go handleNodeConnection(conn, nodemsgchan, nodeaddchan, noderemchan)
	}
}

// handle new connection from client
func handleClientConnection(c net.Conn) {
	defer c.Close()

	var client Client // new client
	client.con = c
	client.ch = make(chan string)
	client.recvch = make(chan string)
	client.rescounter = 0

	buf := bufio.NewReader(c)   // new reader for client
	msg, _, _ := buf.ReadLine() // read search query from client

	if len(Nodes) == 0 { // if there is node connected to server signal not found
		c.Write([]byte(NOTFOUND + "\n"))
		c.Close()
		return
	}

	client.searchquery = make([]byte, 0)
	client.searchquery = msg
	client.id = genClientId() // generate random id for new client
	Clients[client.id] = client

	makeSearch(client) // search the query
	client.writeToClientChan(client.recvch)
}

// implement the search through nodes
func makeSearch(client Client) {
	log.Printf("Client request to search: [%s]\n", string(client.searchquery))
	// preparing message for nodes to send
	readyQuery := SEARCH + SEP + string(client.searchquery) + SEP + client.id
	for _, anode := range Nodes { // sending each node the query
		log.Printf("Sending search query [%s] to node [%d]\n", string(client.searchquery), anode.id)
		go func(mch chan<- string) {
			mch <- readyQuery // adding message to node channel which automatically write to connection
		}(anode.ch)
	}
}

// accept new connection from client
func acceptClientConnections(ln net.Listener) {
	for {
		conn, err := ln.Accept() // accepting the client
		if err != nil {
			log.Fatal(err)
			continue
		}
		log.Printf("New client is connected, addresss: %s\n", conn.RemoteAddr().String())
		go handleClientConnection(conn) // handling client
	}
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Provide port for nodes and client")
	}
	portForNodes := os.Args[1]  // port for listening to nodes
	portForClient := os.Args[2] // port for listening to clients

	// necessary channels
	nodemsgchan := make(chan string)
	nodeaddchan := make(chan Node)
	noderemchan := make(chan Node)

	go handleNodeMessages(nodemsgchan, nodeaddchan, noderemchan)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%s", portForNodes))
	if err != nil {
		log.Fatal("Error in listening for nodes.")
		log.Fatal(err)
		os.Exit(-1)
	}
	log.Printf("server is listening for nodes on port %s.\n", portForNodes)
	defer ln.Close()
	go acceptNodeConnections(ln, nodemsgchan, nodeaddchan, noderemchan)
	// time.Sleep(time.Second * 10)

	// handling clients
	clientLn, errCl := net.Listen("tcp", fmt.Sprintf(":%s", portForClient))
	if errCl != nil {
		log.Fatal("Error in listening for clients.\n")
		panic(errCl)
	}
	log.Printf("Server is listening for clients on port %s.\n", portForClient)
	defer clientLn.Close()
	go acceptClientConnections(clientLn)

	// handling http clients
	// serving static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", rootHandler)          // main page handler
	http.HandleFunc("/search", searchHandler)  // search handler
	http.HandleFunc("/nodes", getNodesHandler) // nodes list handler
	log.Printf("Server is listening for http on port %s.\n", "8080")
	http.ListenAndServe(":8080", nil)
}

// converting nodes info to json string
func getNodesJsonStr() string {
	var nodesInfo string
	nodesInfo = "[ "
	for _, anode := range Nodes { // loop over each node
		nodesInfo += fmt.Sprintf("{\"name\": \"%d\", \"chunks\": \"%s\", \"memory\": \"%s\"}", anode.id, anode.chunkFiles, anode.memoryFiles)
		nodesInfo += ","
	}
	nodesInfo = nodesInfo[:len(nodesInfo)-1] + "]"
	return nodesInfo
}

// nodes list handler
func getNodesHandler(res http.ResponseWriter, req *http.Request) {
	nodesInfo := getNodesJsonStr()                       // json string of nodes info
	res.Header().Set("Content-Type", "application/json") // setting header
	res.WriteHeader(http.StatusOK)                       // ok status
	res.Write([]byte(nodesInfo))                         // sending client the nodes
}

// main page handler
func rootHandler(res http.ResponseWriter, req *http.Request) {
	editTemplate, err := template.ParseFiles("./view/index.html")
	if err != nil {
		log.Fatal("Error in parsing template file")
		log.Fatal(err)
	}
	editTemplate.Execute(res, &Page{title: "CDS-Project"})
}

func searchHandler(res http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		fmt.Fprintf(res, "ParseForm() err: %v", err)
		return
	}
	// getting the search query string from connection
	query := req.FormValue("query")

	// new client
	var client Client
	client.con = nil
	client.ch = make(chan string)
	client.recvch = make(chan string)
	client.rescounter = 0
	client.searchquery = make([]byte, 0)
	client.searchquery = []byte(query)
	client.id = genClientId()
	Clients[client.id] = client

	// perform search
	makeSearch(client)

	var searchResult string
	searchResult = <-client.recvch // waiting on receiving channel
	resultTokens := strings.Split(searchResult, SEP)
	searchResult = <-client.recvch

	delete(Clients, client.id) // delete client

	result := 0
	node := 0
	if resultTokens[0] == FOUND { // if found
		result = 1 // found
		node, _ = strconv.Atoi(resultTokens[1])
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(fmt.Sprintf("{\"result\": %d, \"node\": \"%d\"}", result, node)))
}

type Page struct {
	title string
}

// letters to generate random string
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// return random string of size n
func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// function to return new id for client of 13 letters
func genClientId() string {
	return RandString(13)
}
