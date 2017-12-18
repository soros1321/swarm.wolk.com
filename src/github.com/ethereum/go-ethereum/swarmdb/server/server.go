package server

import (
	"bufio"
	//"bytes"
	//"encoding/json"
	"fmt"
	"io"
	//"io/ioutil"
	"github.com/ethereum/go-ethereum/log"
	"net"
	//"os"
	common "github.com/ethereum/go-ethereum/swarmdb"
	//"strconv"
	"sync"
)

type ServerConfig struct {
	Addr string
	Port string
}

type Client struct {
	conn     net.Conn
	incoming chan *common.IncomingInfo
	outgoing chan string
	reader   *bufio.Reader
	writer   *bufio.Writer
	table    *common.Table // holds ownerID, tableName
}

type TCPIPServer struct {
	swarmdb  *common.SwarmDB
	listener net.Listener
	conn     chan net.Conn
	incoming chan *common.IncomingInfo
	outgoing chan string
	clients  []*Client
	lock     sync.Mutex
}

func NewTCPIPServer(swarmdb *common.SwarmDB, l net.Listener) *TCPIPServer {
	sv := new(TCPIPServer)
	sv.listener = l
	sv.clients = make([]*Client, 0)
	sv.conn = make(chan net.Conn)
	sv.incoming = make(chan *common.IncomingInfo)
	sv.outgoing = make(chan string)
	sv.swarmdb = swarmdb
	return sv
}

func StartTCPIPServer(swarmdb *common.SwarmDB, config *ServerConfig) {
	log.Debug(fmt.Sprintf("tcp StartTCPIPServer"))

	//listen, err := net.Listen("tcp", config.Port)
	l, err := net.Listen("tcp", ":2000")
	log.Debug(fmt.Sprintf("tcp StartTCPIPServer with 2000"))

	svr := NewTCPIPServer(swarmdb, l)
	if err != nil {
		//log.Fatal(err)
		log.Debug(fmt.Sprintf("err"))
	}
	//defer svr.listener.Close()

	svr.listen()
	for {
		conn, err := svr.listener.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}

		challenge := RandStringRunes(64)
		nonce := RandStringRunes(48)
		s := fmt.Sprintf(`{"challenge":"%s","nonce":"%s"}\n`, challenge, nonce)
		conn.Write([]byte(s))
		svr.conn <- conn
	}
	if err != nil {
		//	log.Fatal(err)
		log.Debug(fmt.Sprintf("err"))
	}
	defer svr.listener.Close()
}

func newClient(connection net.Conn) *Client {
	writer := bufio.NewWriter(connection)
	reader := bufio.NewReader(connection)
	client := &Client{
		conn:     connection,
		incoming: make(chan *common.IncomingInfo),
		outgoing: make(chan string),
		reader:   reader,
		writer:   writer,
		//databases: make(map[string]map[string]*common.Database),
	}
	go client.read()
	//go client.write()
	return client
}

func (client *Client) read() {
	for {
		line, err := client.reader.ReadString('\n')
		if err == io.EOF {
			client.conn.Close()
			break
		}
		if err != nil {
			////////
		}
		incoming := new(common.IncomingInfo)
		incoming.Data = line
		incoming.Address = client.conn.RemoteAddr().String()
		//client.incoming <- line
		client.incoming <- incoming
		fmt.Printf("[%s]Read:%s", client.conn.RemoteAddr(), line)
	}
}
func (client *Client) write() {
	for data := range client.outgoing {
		client.writer.WriteString(data)
		//client.writer.Write(data)
		client.writer.Flush()
		fmt.Printf("[%s]Write:%s\n", client.conn.RemoteAddr(), data)
	}
}

func (svr *TCPIPServer) addClient(conn net.Conn) {
	fmt.Printf("\nConnection Added")
	fmt.Fprintf(conn, "Your Connection Added\n")
	client := newClient(conn)
	/// this one is not good. need to change it
	svr.clients = append(svr.clients, client)
	go func() {
		for {
			svr.incoming <- <-client.incoming
			client.outgoing <- <-svr.outgoing
		}
	}()
}

func RandStringRunes(n int) string {
	// var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	var letterRunes = []rune("0123456789abcdef")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func handleRequest(conn net.Conn, nonce string, challenge string) {
	// Make a buffer to hold incoming data.
	buf := make([]byte, 1024)
	// Read the incoming connection into the buffer.
	reqLen, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading:", err.Error())
	}
	
	// this should be the signed challenge, verify using valid_response
	resp := string(buf)
	resp = strings.Trim(resp, "\n")
	if valid_response(resp, nonce, challenge) {
		resp = "VALID"
	} else {
		resp = "INVALID"
	}
	s := fmt.Sprintf("%d:%s", reqLen, resp)
	conn.Write([]byte(s))
	// Close the connection when you're done with it.
	conn.Close()
}

func (svr *TCPIPServer) TestAddClient(owner string, tablename string, primary string) {
	//testConn := svr.NewConnection()
	client := newClient(nil) //testConn)
	client.table = svr.swarmdb.NewTable(owner, tablename)
	//client.table.SetPrimary( primary )
	svr.clients = append(svr.clients, client)
}

func (svr *TCPIPServer) listen() {
	go func() {
		for {
			select {
			case conn := <-svr.conn:
				svr.addClient(conn)
			case data := <-svr.incoming:
				fmt.Printf("\nIncoming Data [%+v]", data)
				resp := svr.swarmdb.SelectHandler(data)
				fmt.Fprintf(svr.clients[0].conn, resp)
				svr.outgoing <- resp
				//default:
				// fmt.Println("\nIn DEFAULT     .")
			}
		}
	}()
}

func (svr *TCPIPServer) NewConnection() (err error) {
	ownerID := "owner1"
	tableName := "testtable"
	svr.swarmdb.NewTable(ownerID, tableName)

	// svr.table = table

	return nil
}