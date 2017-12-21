package main

import (
	"encoding/json"
	"fmt"
	swarmdb "github.com/ethereum/go-ethereum/swarmdb"
"github.com/ethereum/go-ethereum/swarmdb/keymanager"
	"github.com/rs/cors"
	"net"
	"net/http"
	"strings"
)

// ServerConfig is the basic configuration needed for the HTTP server and also
// includes CORS settings.
type ServerConfig struct {
	Addr       string
	CorsString string
}

type HTTPServer struct {
	swarmdb  *swarmdb.SwarmDB
	listener net.Listener
	//keymanager keymanager.KeyManager
	//lock       sync.Mutex
}

type SwarmDBReq struct {
	protocol string
	table    string
	key      string
}

type DataReq struct {
	RequestType string `json:"requesttype,omitempty"`
	Table       string `json:"table,omitempty"`
	Key         string `json:"key,omitempty"`
}

func parsePath(path string) (swdbReq SwarmDBReq, err error) {
	pathParts := strings.Split(path, "/")
	if len(pathParts) < 2 {
		return swdbReq, fmt.Errorf("Invalid Path")
	} else {
		swdbReq.protocol = pathParts[1]
		swdbReq.table = pathParts[2]
		swdbReq.key = pathParts[3]
	}
	return swdbReq, nil
}

func StartHttpServer(config *ServerConfig) {
	fmt.Println("\nstarting http server")
	httpSvr := new(HTTPServer)
	httpSvr.swarmdb = swarmdb.NewSwarmDB()
	var allowedOrigins []string
	/*
	   for _, domain := range strings.Split(config.CorsString, ",") {
	*/
	allowedOrigins = append(allowedOrigins, "corsdomain")
	// }
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"POST", "GET", "DELETE", "PATCH", "PUT"},
		MaxAge:         600,
		AllowedHeaders: []string{"*"},
	})
	//sk, pk := GetKeys()
	hdlr := c.Handler(httpSvr)

	fmt.Printf("\nRunning ListenAndServer")
	//go http.ListenAndServe(config.Addr, hdlr)
	http.ListenAndServe(config.Addr, hdlr)
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	if r.Method == "OPTIONS" {
		return
	}
	fmt.Println("HTTP %s request URL: '%s', Host: '%s', Path: '%s', Referer: '%s', Accept: '%s'", r.Method, r.RequestURI, r.URL.Host, r.URL.Path, r.Referer(), r.Header.Get("Accept"))
	swReq, _ := parsePath(r.URL.Path)
	//Parse BodyContent

	var dataReq DataReq
	if swReq.protocol != "swarmdb:" {
		//Invalid Protocol: Throw Error
		fmt.Fprintf(w, "The protocol sent in: %s is invalid | %+v\n", swReq.protocol, swReq)
	} else {
		if r.Method == "GET" {
			fmt.Fprintf(w, "Processing [%s] protocol request with Body of () \n", swReq.protocol)
			dataReq.RequestType = "Get"
			dataReq.Table = swReq.table
			dataReq.Key = swReq.key
			retJson, _ := json.Marshal(dataReq)
			//Redirect to SelectHandler after "building" GET RequestOption
			s.swarmdb.SelectHandler(keymanager.WOLKSWARMDB_ADDRESS, string(retJson))
		} else if r.Method == "POST" {
			bodyContent := r.Body
			fmt.Fprintf(w, "Processing [%s] protocol request with Body of (%s) \n", swReq.protocol, bodyContent)
			//READ parsed body content to get the RequestType
			//Retrieve body content
			//Redirect to SelectHandler after "building" appropriate RequestOption
		}
	}
}

/*
func handleRequest(conn net.Conn, svr *TCPIPServer) {
        // generate a random 32 byte challenge (64 hex chars)
        // challenge = "27bd4896d883198198dc2a6213957bc64352ea35a4398e2f47bb67bffa5a1669"
        challenge := RandStringRunes(64)

        reader := bufio.NewReader(conn)
        writer := bufio.NewWriter(conn)

        client := &Client{
                conn:   conn,
                reader: reader,
                writer: writer,
                svr:    svr,
        }

        fmt.Fprintf(writer, "%s\n", challenge)
        writer.Flush()

        fmt.Printf("accepted connection [%s]\n", challenge);
        // Make a buffer to hold incoming data.
        //buf := make([]byte, 1024)
        // Read the incoming connection into the buffer.
        // reqLen, err := conn.Read(buf)
        resp, err := reader.ReadString('\n')
        if err != nil {
                fmt.Println("Error reading:", err.Error())
        } else {
                resp = strings.Trim(resp, "\r")
                resp = strings.Trim(resp, "\n")
        }

        // this should be the signed challenge, verify using valid_response
        challenge_bytes, err2 := hex.DecodeString(challenge)
        if err2 != nil {
                fmt.Printf("ERR decoding challenge:[%s]\n", challenge)
        }
        // resp = "6b1c7b37285181ef74fb1946968c675c09f7967a3e69888ee37c42df14a043ac2413d19f96760143ee8e8d58e6b0bda4911f642912d2b81e1f2834814fcfdad700"
        // fmt.Printf("BUF %d: %v\n", len([]byte(resp)), []byte(resp))

        response_bytes, err3 := hex.DecodeString(resp)
        // fmt.Printf("Response: [%d] %s \n", len(response_bytes), resp);
        if err3 != nil {
                fmt.Printf("ERR decoding response:[%s]\n", resp)
        }

        verified, err := svr.keymanager.VerifyMessage(challenge_bytes, response_bytes)
        if err != nil {
                resp = "ERR"
        }  else if verified {
                resp = "OK"
        } else {
                resp = "INVALID"
        }
        fmt.Printf("%s C: %x R: %x\n", resp, challenge_bytes, response_bytes);
        // fmt.Fprintf(writer, resp)
        writer.Flush()
        if ( resp == "OK" ) {
                for {
                        str, err := client.reader.ReadString('\n')
                        if err == io.EOF {
                                conn.Close()
                                break
                        }
                        if ( true ) {
                                resp, err := svr.swarmdb.SelectHandler(keymanager.WOLKSWARMDB_ADDRESS, str)
                                if err != nil {
                                        s := fmt.Sprintf("ERR: %s\n", err)
                                        writer.WriteString(s)
                                        writer.Flush()
                                } else {
                                        fmt.Printf("Read: [%s] Wrote: [%s]\n", str, resp)
                                        fmt.Fprintf(client.writer, resp)
                                }
                        } else {
                                writer.WriteString("OK\n")
                                writer.Flush()
                        }
                }
        } else {
                conn.Close()
        }
        // Close the connection when you're done with it.
}
*/

func main() {
	fmt.Println("Launching server...")
	listenerAddress := "localhost"
	listenerPort := "8100"

	// start swarm http proxy server
	addr := net.JoinHostPort(listenerAddress, listenerPort)
	StartHttpServer(&ServerConfig{
		Addr:       addr,
		CorsString: "",
	})
	fmt.Println("\nAfter StartHttpServer Addr")
}
