/*
   Copyright 2018 Jook.co

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jookco/pgproxy.v1/common"
	"github.com/jookco/pgproxy.v1/config"
	"github.com/jookco/pgproxy.v1/connect"
	"github.com/jookco/pgproxy.v1/pool"
	"github.com/jookco/pgproxy.v1/protocol"
	"github.com/jookco/pgproxy.v1/utils/log"
	"github.com/mediocregopher/radix.v3"
)

const (
	sendTypeRead  = "read"
	sendTypeWrite = "write"
)

type Proxy struct {
	writeNodes     *writeNodeMap
	readNodes      chan *pool.Pool
	startTime      int
	stats          *nodeStatsMap
	delayQueryChan chan delayQueryChanStruct
	nodeReady      *nodeReadyMap
	nodeReadyChan  chan nodeReadyChanStruct
	nodesOffline   *offlineNodeMap
	redisDB        *radix.Pool
	restoreLock    map[string]*sync.Mutex
	cleanLock      map[string]*sync.Mutex
}

type nodeReadyChanStruct struct {
	Opt  string
	Node common.Node
}

type ProxyStats struct {
	QueryCount  uint64
	SelectCount uint64
	OtherCount  uint64
}

type delayQueryChanStruct struct {
	Name             string
	Data             []byte
	Opt              string
	QueryKey         string
	PreparedQueryKey string
}

type nodePoolStruct struct {
	Conn             net.Conn
	Cp               *pool.Pool
	Name             string
	Data             chan []byte
	PreparedQueryKey string
	WriteDataChannel *writeDataChannel
	P                *Proxy
	Read             bool
	Prepared         bool
	ResWg            *sync.WaitGroup
}

func (w *nodePoolStruct) WriteData(data []byte) {
	log.Infof("WriteData Receive:%v", w.Conn)
	w.Data <- data
}

func (w *nodePoolStruct) Close() {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteByte(protocol.ConnCloseMessageType)
	w.Data <- buf.Bytes()
}

func (w *nodePoolStruct) SendData() {
	connError := false
	defer func() {
		if !connError {
			log.Infof("SendData Returned %v", w.Conn)
			w.Cp.Return(w.Conn)
		} else {
			log.Infof("backend Closed %v", w.Conn)
			w.Conn.Close()
		}
	}()
	var messageType byte
	var message []byte
	var length int
	buf := bytes.NewBuffer([]byte{})
	allBuf := bytes.NewBuffer([]byte{})
	log.Debugf("%v SendData Start Listen %v", time.Now().UnixNano(), w.Conn)
	for {
		select {
		case data := <-w.Data:
			log.Infof("%v SendData Call Data  Start %v Data:%v", time.Now().UnixNano(), w.Conn, string(data))
			if protocol.GetMessageType(data) == protocol.ConnCloseMessageType {
				connError = false
				return
			}
			if !w.Read {
				go w.P.stats.Add(w.Name, common.StatsTypeOther)
			} else {
				go w.P.stats.Add(w.Name, common.StatsTypeSelect)
			}

			messageType = protocol.GetMessageType(data)

			var err error
			if _, err = connect.Send(w.Conn, data); err != nil {
				log.Errorf("Error sending message to backend %s", w.Conn.RemoteAddr())
				log.Errorf("Error: %s", err.Error())
				connError = true
				if err == io.EOF {
					go func() {
						node := config.GetNodeByHostPort(w.Name)
						w.P.nodeReadyChan <- nodeReadyChanStruct{Node: node, Opt: "add"}
					}()
				}
				go func() {
					w.WriteDataChannel.Error <- err
				}()
				w.ResWg.Done()
				return
			}
			w.ResWg.Done()

			/*
			 * Continue to read from the backend until a 'ReadyForQuery' message is
			 * is found.
			 */

			var done bool
			for !done {
				message, length, err = connect.Receive(w.Conn)
				if err != nil {
					log.Errorf("Error receiving response from backend2 %s", w.Conn.RemoteAddr())
					log.Errorf("Error: %s", err.Error())
					connError = true
					if err == io.EOF {
						go func() {
							node := config.GetNodeByHostPort(w.Name)
							w.P.nodeReadyChan <- nodeReadyChanStruct{Node: node, Opt: "add"}
						}()
					}
					go func() {
						w.WriteDataChannel.Error <- err
					}()
					return
				}

				allBuf.Write(message[:length])
				buf.Write(message[:length])

				/*
				 * Examine all of the messages in the buffer and determine if any of
				 * them are a ReadyForQuery message.
				 */
				bufLen := buf.Len()
				for start := 0; start < bufLen; {
					messageType = protocol.GetMessageType(buf.Bytes()[start:])
					messageLength := int32(len(buf.Bytes()[start:]))
					if len(buf.Bytes()[start:]) >= 5 {
						messageLength = protocol.GetMessageLength(buf.Bytes()[start:])
					}

					// log.Infof("%v Node:'%v' HandleWriteData start messageType %v messageLength:%v", time.Now().UnixNano(), w.Conn, string(messageType), messageLength)
					/*
					 * Calculate the next start position, add '1' to the message
					 * length to account for the message type.
					 */

					oldStart := start

					start = (start + int(messageLength) + 1)
					if start >= bufLen {
						log.Debugf("old %d start %d length %d", oldStart, start, bufLen)
						newBytes := buf.Bytes()[oldStart:]
						buf.Reset()
						buf.Write(newBytes)
						messageType = protocol.GetMessageType(buf.Bytes())
						break
					}
					// log.Infof("Node:'%v' HandleWriteData messageType %v messageLength:%v %v", w.Name, string(messageType), messageLength, string(buf.Bytes()[oldStart:start]))
				}

				done = (messageType == protocol.ReadyForQueryMessageType)
				// log.Infof("Node:'%v' Done:%v HandleWriteData END =========================================================================================== ", w.Name, done)
			}
			buf.Reset()

			if !w.Read {
				tempKey := fmt.Sprintf("delayQuery_%s_%s_tmp", w.Name, w.PreparedQueryKey)
				w.P.redisDB.Do(radix.Cmd(nil, "DECR", tempKey))
			}

			w.WriteDataChannel.WriteData(allBuf.Bytes(), w)
			allBuf.Reset()
		}
	}
}

func NewProxy() *Proxy {
	p := &Proxy{
		startTime:      int(time.Now().Unix()),
		nodeReadyChan:  make(chan nodeReadyChanStruct),
		delayQueryChan: make(chan delayQueryChanStruct),
		restoreLock:    make(map[string]*sync.Mutex),
		cleanLock:      make(map[string]*sync.Mutex),
	}

	err := p.init()
	if err != nil {
		log.Errorf("init err:%v", err)
		return nil
	}

	go p.delayQueryOpt()

	go p.startHealthCheck()

	return p
}
func (p *Proxy) closeNode(hostPort string) {
	_node := p.writeNodes.Get(hostPort)
	if _node != nil {
		_node.Close()
		p.writeNodes.Del(hostPort)
	}
	loadedLen := 0
	readNodesLen := len(p.readNodes)
	for loadedLen < readNodesLen {
		select {
		case node := <-p.readNodes:
			loadedLen++
			if node.Name == hostPort {
				node.Close()
				return
			}
			p.returnReadNode(node)
		default:
			return
		}
	}
}

func (p *Proxy) Close() {
	log.Debugf("Proxy Close")
	for _, v := range p.writeNodes.Map {
		v.Close()
	}

	for {
		select {
		case _node := <-p.readNodes:
			_node.Close()
		default:
			return
		}
	}
}

func (p *Proxy) createConnection(hostPort string, credentials common.Credentials) (connection net.Conn, err error) {
	connection, err = connect.Connect(hostPort)
	if err != nil {
		return
	}
	var length int
	startupMessage := protocol.CreateStartupMessage(credentials.Username, credentials.Database, credentials.Options)
	connection.Write(startupMessage)
	response := make([]byte, 4096)
	length, err = connection.Read(response)
	if err != nil {
		return
	}

	authenticated := connect.HandleAuthenticationRequest(connection, response[:length])
	if !authenticated {
		err = errors.New(fmt.Sprintf("WriteNode:'%v' Authentication failed!", hostPort))
		return
	}
	return
}

func (p *Proxy) makeConnection(node common.Node) (writePoolsLen, readPoolsLen int, nodeReady bool) {
	writePools := config.GetNodeWritePools()
	nodeReady = false
	credentials := config.GetCredentials()
	if writePools > 0 && node.Role == common.NODE_ROLE_WRITER {
		writePool := pool.NewPool(node.HostPort, writePools)
		for i := 0; i < writePools; i++ {
			connection, err := p.createConnection(node.HostPort, credentials)
			if err != nil {
				log.Debugf("WriteNode Err:%v", err)
				return
			}

			log.Debugf("Successfully connected to WriteNode:'%s'", node.HostPort)
			writePool.Add(connection)
		}

		writePoolsLen = writePool.Len()
		if writePoolsLen > 0 {
			restorePool := writePool.Next()
			log.Infof("Start Restore")
			if err := p.restoreNodeData(node.HostPort, restorePool); err != nil {
				return
			}
			log.Infof("Start Restore OK")
			writePool.Return(restorePool)
			p.writeNodes.Set(node.HostPort, writePool)
			nodeReady = true
		}
		if writePoolsLen == 0 {
			return
		}
	}
	if node.ReadPools > 0 {
		readPool := pool.NewPool(node.HostPort, node.ReadPools)
		var readWg sync.WaitGroup
		readWg.Add(node.ReadPools)
		for i := 0; i < node.ReadPools; i++ {
			connection, err := p.createConnection(node.HostPort, credentials)
			if err != nil {
				log.Debugf("ReadNode Err:%v", err)
				return
			}

			log.Debugf("Successfully connected to ReadNode'%s'", node.HostPort)
			readPool.Add(connection)
		}
		readPoolsLen = readPool.Len()
		if readPoolsLen > 0 {
			nodeReady = true
			p.readNodes <- readPool
		}
	}
	return
}

func (p *Proxy) init() error {
	redisConfig := config.GetRedisConfig()
	redisPool, err := radix.NewPool("tcp", redisConfig.HostPort, 10, nil)
	if err != nil {
		log.Errorf("pool err:%v", err)
		return
	}

	nodes := config.GetNodes()
	p.nodesOffline = newOfflineNodeMap(len(nodes))

	p.stats = newNodeStatsMap()
	p.nodeReady = newNodeReadyMap()

	p.redisDB = redisPool
	p.redisDB.Do(radix.Cmd(nil, "FLUSHALL"))
	p.redisDB.Do(radix.Cmd(nil, "SELECT", redisConfig.Database))
	log.Infof("Redis Server Connected %s", redisConfig.HostPort)

	/* Initialize pool structures */
	p.writeNodes = newWriteNodeMap()
	p.readNodes = make(chan *pool.Pool, len(nodes))
	var wg sync.WaitGroup
	wg.Add(len(nodes))
	for _, node := range nodes {
		// init stats map
		p.stats.Map[node.HostPort] = &ProxyStats{}
		// init locks
		p.restoreLock[node.HostPort] = &sync.Mutex{}
		p.cleanLock[node.HostPort] = &sync.Mutex{}
		p.nodeReady.Set(node.HostPort, false)
		_node := node
		go func() {
			defer wg.Done()
			log.Infof("Node:'%v' Initialzing...", _node.HostPort)
			writePoolsLen, readPoolsLen, nodeReady := p.makeConnection(_node)
			p.nodeReady.Set(_node.HostPort, nodeReady)
			log.Infof("Node:'%v' WritePools:%d ReadPools:%d Ready:%v", _node.HostPort, writePoolsLen, readPoolsLen, nodeReady)
			if !nodeReady {
				go func() {
					p.nodeReadyChan <- nodeReadyChanStruct{Node: _node, Opt: "add"}
				}()
			}
		}()
	}
	wg.Wait()
}

// Get the read node
func (p *Proxy) getReadNode() *pool.Pool {
	loadedLen := 0
	nodesLen := len(p.readNodes)
	for loadedLen < nodesLen {
		select {
		case _node := <-p.readNodes:
			if !p.nodeReady.Get(_node.Name) {
				go func() {
					node := config.GetNodeByHostPort(_node.Name)
					if node.HostPort == "" {
						return
					}
					p.nodeReadyChan <- nodeReadyChanStruct{Node: node, Opt: "add"}
				}()
				loadedLen++
				continue
			} else if _node.Len() == 0 {
				p.returnReadNode(_node)
				continue
			}
			return _node
		}
	}
	return nil
}

// Return the read node
func (p *Proxy) returnReadNode(pl *pool.Pool) {
	p.readNodes <- pl
}

func (p *Proxy) addDelayQuery(data []byte, queryKey, preparedQueryKey string) {
	p.delayQueryChan <- delayQueryChanStruct{Data: data, Opt: "add", QueryKey: queryKey, PreparedQueryKey: preparedQueryKey}
}

// HandleConnection handle an incoming connection to the proxy
func (p *Proxy) HandleConnection(client net.Conn) {
	nodes := config.GetNodes()
	log.Debugf("%v Start", time.Now().UnixNano())
	pools := make(map[string]map[string]*nodePoolStruct, 2)
	pools[sendTypeRead] = make(map[string]*nodePoolStruct, 1)
	pools[sendTypeWrite] = make(map[string]*nodePoolStruct, len(nodes))
	defer func() {
		for _type, v := range pools {
			for _, p := range v {
				p.Close()
				if _, ok := pools[_type][p.Name]; ok {
					delete(pools[_type], p.Name)
				}
			}
		}
		client.Close()
		log.Debugf("%v client closed", time.Now().UnixNano())
	}()
	/* Get the client startup message. */
	message, length, err := connect.Receive(client)
	if err != nil {
		log.Error("Error receiving startup message from client.")
		log.Errorf("Error: %s", err.Error())
		return
	}
	log.Debugf("HandleConnection client:%v %v", client, string(message[:length]))

	/* Get the protocol from the startup message.*/
	version := protocol.GetVersion(message)
	/* Handle the case where the startup message was an SSL request. */
	if version == protocol.SSLRequestCode {
		sslResponse := protocol.NewMessageBuffer([]byte{})

		/* Determine which SSL response to send to client. */
		creds := config.GetCredentials()
		if creds.SSL.Enable {
			sslResponse.WriteByte(protocol.SSLAllowed)
		} else {
			sslResponse.WriteByte(protocol.SSLNotAllowed)
		}

		/*
		 * Send the SSL response back to the client and wait for it to send the
		 * regular startup packet.
		 */
		connect.Send(client, sslResponse.Bytes())
		if creds.SSL.Enable {
			/* Upgrade the client connection if required. */
			client = connect.UpgradeServerConnection(client)
		}

		/*
		 * Re-read the startup message from the client. It is possible that the
		 * client might not like the response given and as a result it might
		 * close the connection. This is not an 'error' condition as this is an
		 * expected behavior from a client.
		 */
		if message, length, err = connect.Receive(client); err == io.EOF {
			log.Debugf("The client closed the connection.")
			return
		}
	}

	/*
	 * Validate that the client username and database are the same as that
	 * which is configured for the proxy connections.
	 *
	 * If the the client cannot be validated then send an appropriate PG error
	 * message back to the client.
	 */
	if !connect.ValidateClient(message) {
		pgError := protocol.Error{
			Severity: protocol.ErrorSeverityFatal,
			Code:     protocol.ErrorCodeInvalidAuthorizationSpecification,
			Message:  "Authentication failed!",
		}

		connect.Send(client, pgError.GetMessage())
		log.Debugf("Could not validate client")
		return
	}

	/* Authenticate the client against the appropriate backend. */

	log.Debugf("Client: %v - authenticating", client)
	authNodeHostPort := p.nodeReady.GetOK()
	if authNodeHostPort == "" {
		pgError := protocol.Error{
			Severity: protocol.ErrorSeverityFatal,
			Code:     protocol.ErrorCodeSQLServerRejectedEstablishementOfSQLConnection,
			Message:  "Node Not Ready",
		}

		connect.Send(client, pgError.GetMessage())
		return
	}

	authenticated, err := connect.AuthenticateClient(client, message, length, authNodeHostPort)
	/* If the client could not authenticate then go no further. */
	if err == io.EOF || err != nil {
		pgError := protocol.Error{
			Severity: protocol.ErrorSeverityFatal,
			Code:     protocol.ErrorCodeInvalidAuthorizationSpecification,
			Message:  "Server Not Ready!",
		}

		connect.Send(client, pgError.GetMessage())
		return
	} else if !authenticated {
		log.Errorf("Client: %s - authentication failed", client.RemoteAddr())
		log.Errorf("Error: %s", err.Error())
		return
	}
	log.Debugf("%v Client: %s - authentication successful", time.Now().UnixNano(), client.RemoteAddr())

	/* Process the client messages for the life of the connection. */
	var readNode *pool.Pool // The connection pool in use
	var read bool
	var nodeName string
	var preparedQuery bool
	var preparedQueryBackend net.Conn
	var queryKey string
	var preparedQueryKey string
	var inTransaction bool
	// poolConfig := config.GetWriteNodeConfig()
	writePoolsDelayTime := 1 * time.Nanosecond
	var resWg sync.WaitGroup
	messageBuf := bytes.NewBuffer([]byte{})
	var messageType byte
	var totalLen int32
	var sendType string
	var resChan *writeDataChannel
	for {
		totalLen = 0
		for {
			message, length, err = connect.Receive(client)
			if err != nil {
				switch err {
				case io.EOF:
					log.Debugf("Client: %s - closed the connection", client.RemoteAddr())
				default:
					log.Errorf("Error reading from client connection %s", client.RemoteAddr())
					log.Errorf("Error: %s", err.Error())
				}
				return
			}

			messageBuf.Write(message[:length])
			messageLength := protocol.GetMessageLength(message)
			if totalLen == 0 {
				totalLen = messageLength
			}
			if int(totalLen+1) <= length {
				break
			}

			if int(totalLen+1) > messageBuf.Len() {
				continue
			}

			if int(totalLen+1) <= messageBuf.Len() {
				break
			}

			messageType = protocol.GetMessageType(messageBuf.Bytes()[int(totalLen+1):])
			if messageType == protocol.ExcuteMessageType {
				if messageBuf.Len() < int(totalLen+6) {
					continue
				}

				excuteMessageLength := protocol.GetMessageLength(messageBuf.Bytes()[int(totalLen+1):])
				if messageBuf.Len() >= int(excuteMessageLength+totalLen+2) {
					break
				}
				continue
			}
			break
		}
		message = messageBuf.Bytes()

		messageType = protocol.GetMessageType(message)
		log.Debugf("%v Client:%v Message %v", time.Now().UnixNano(), client, string(message))

		if messageType == protocol.TerminateMessageType {
			log.Infof("Client: %v - disconnected", client)
			return
		}
		if !inTransaction {
			inTransaction = getQueryIsTransaction(message)
			if inTransaction {
				read = false
				preparedQueryBackend = nil
				preparedQuery = true
				preparedQueryKey = time.Now().String()
			}
		}

		if messageType == protocol.ParseMessageType {
			if !inTransaction {
				preparedQuery = true
				read = getQueryIsRead(message)
				preparedQueryBackend = nil
				preparedQueryKey = time.Now().String()
			}
		} else {
			if !preparedQuery {
				read = getQueryIsRead(message)
				preparedQueryKey = time.Now().String()
			}
		}

		if getQueryIsTransactionFinish(message) {
			inTransaction = false
			preparedQuery = false
		}
		resChan = &writeDataChannel{C: make(chan []byte), Error: make(chan error)}

		if !read {
			sendType = sendTypeWrite
			queryKey = strconv.Itoa(int(time.Now().UnixNano()))
			go p.addDelayQuery(message[:length], queryKey, preparedQueryKey)

			// go func() {
			// 	p.delayQueryChan <- delayQueryChanStruct{Data: message[:length], Opt: "add", QueryKey: queryKey, PreparedQueryKey: preparedQueryKey}
			// }()

			if preparedQuery && preparedQueryBackend != nil {
				resChan.backend = preparedQueryBackend
			}
			for _, v := range p.writeNodes.Map {
				if _, ok := pools[sendTypeWrite][v.Name]; ok {
					continue
				}
				if !p.nodeReady.Get(v.Name) || v.Len() == 0 {
					continue
				}
				writePool := &nodePoolStruct{Conn: v.Next(), Cp: v, Name: v.Name, Data: make(chan []byte), P: p, Read: read}
				go writePool.SendData()
				pools[sendTypeWrite][v.Name] = writePool
				log.Infof("%v writeNode '%v' Len:%v", time.Now().UnixNano(), writePool.Conn, v.Len(), client)
			}
			if len(pools[sendTypeWrite]) == 0 {
				pgError := protocol.Error{
					Severity: protocol.ErrorSeverityFatal,
					Code:     protocol.ErrorCodeSQLServerRejectedEstablishementOfSQLConnection,
					Message:  "Node Not Ready",
				}

				connect.Send(client, pgError.GetMessage())
				return
			}

		} else {
			sendType = sendTypeRead
			if len(pools[sendTypeRead]) == 0 {
				readNode = p.getReadNode()
				log.Debugf("new readNode:%v", readNode)
				nodeName = readNode.Name
				if !p.nodeReady.Get(nodeName) {
					pgError := protocol.Error{
						Severity: protocol.ErrorSeverityFatal,
						Code:     protocol.ErrorCodeSQLServerRejectedEstablishementOfSQLConnection,
						Message:  "Node Not Ready",
					}

					connect.Send(client, pgError.GetMessage())
					return
				}
				readPool := &nodePoolStruct{Conn: readNode.Next(), Cp: readNode, Name: nodeName, Data: make(chan []byte), P: p, Read: read}
				go readPool.SendData()
				pools[sendTypeRead][nodeName] = readPool
				p.returnReadNode(readNode)
				log.Debugf("%v readNode '%v' Len:%v Client:%v", time.Now().UnixNano(), readPool.Conn, readNode.Len(), client)
			}
		}

		resWg.Add(1)
		go func() {
			defer func() {
				resWg.Done()
			}()
			errorLength := 0
			for {
				select {
				case res := <-resChan.C:
					if _, err = connect.Send(client, res); err != nil {
						log.Errorf("Error sending response to client %s", client.RemoteAddr())
						log.Errorf("Error: %s", err.Error())
						return
					}

					if preparedQueryBackend == nil {
						preparedQueryBackend = resChan.backend
					}
					if !read && !preparedQuery {
						for k := range pools[sendTypeWrite] {
							delete(pools[sendTypeWrite], k)
						}
					}
					if !read {
						time.Sleep(writePoolsDelayTime)
					}

					return
				case <-resChan.Error:
					errorLength++
					errorLengthLimit := len(pools[sendType])
					log.Debugf("errorLengthLimit:%v errorLength:%v", errorLengthLimit, errorLength)
					if errorLength == errorLengthLimit {
						for hp := range pools[sendType] {
							delete(pools[sendType], hp)
						}
						pgError := protocol.Error{
							Severity: protocol.ErrorSeverityFatal,
							Code:     protocol.ErrorCodeSQLServerRejectedEstablishementOfSQLConnection,
							Message:  "Node Not Ready",
						}

						connect.Send(client, pgError.GetMessage())
						client.Close()
						return
					}
				}
			}
		}()
		log.Infof("%v pools[%v] %v CLient:%v preparedQuery:%v transaction:%v", time.Now().UnixNano(), sendType, pools[sendType], client, preparedQuery, inTransaction)
		for _, v := range pools[sendType] {
			resWg.Add(1)
			log.Infof("add 1---------------------")
			_pool := v
			_message := messageBuf.Bytes()
			_pool.PreparedQueryKey = preparedQueryKey
			_pool.WriteDataChannel = resChan
			_pool.Prepared = preparedQuery
			_pool.ResWg = &resWg
			go _pool.WriteData(_message)
		}
		resWg.Wait()
		messageBuf.Reset()
		log.Debugf("resWg Done!")
	}
}

type writeDataChannel struct {
	C          chan []byte
	Error      chan error
	backend    net.Conn
	dataWrited bool
}

func (w *writeDataChannel) WriteData(data []byte, n *nodePoolStruct) {
	defer func() {
		if !n.Read && !n.Prepared {
			log.Debugf("writeDataChannel Closed %v", n.Conn)
			go n.Close()
		}
	}()
	log.Debugf("Node:'%v' Read:%v prepared:%v writeDataChannel WriteData EX", n.Conn, n.Read, n.Prepared)
	// log.Infof("%v Node:'%v' Read:%v writeDataChannel WriteData:%v", time.Now().UnixNano(), n.Conn, n.Read, string(data))
	if w.backend != nil && w.backend != n.Conn {
		return
	}
	if w.backend == nil && n.Prepared {
		w.backend = n.Conn
	}
	if !w.dataWrited {
		w.dataWrited = true
		log.Debugf("Node:'%v' Read:%v prepared:%v writeDataChannel WriteData OK", n.Conn, n.Read, n.Prepared)
		w.C <- data
	}
}

func (p *Proxy) startHealthCheck() {
	nodes := config.GetNodes()
	healthConfig := config.GetHealthCheckConfig()
	credentials := config.GetCredentials()

	healthTicker := time.NewTicker(time.Second * healthConfig.Delay)
	var wg sync.WaitGroup

	nodeProcess := make(map[string]bool)
	nodeProcessLock := sync.Mutex{}

	for range healthTicker.C {
		for _, n := range nodes {
			if _, ok := nodeProcess[n.HostPort]; ok {
				continue
			}
			nodeProcess[n.HostPort] = true
			wg.Add(1)
			node := n
			go func() {
				defer wg.Done()
				if p.nodeReady.Get(node.HostPort) {
					go p.cleanRestoreData(node.HostPort)
				}
				connection, err := p.createConnection(node.HostPort, credentials)
				if err != nil {
					p.nodeReady.Set(node.HostPort, false)
					if !p.nodesOffline.Has(node.HostPort) {
						p.closeNode(node.HostPort)
						p.nodesOffline.Set(node)
					}
					nodeProcessLock.Lock()
					delete(nodeProcess, node.HostPort)
					nodeProcessLock.Unlock()
					return
				}
				connection.Close()
				if p.nodesOffline.Has(node.HostPort) {
					_, _, _ready := p.makeConnection(node)
					p.nodeReady.Set(node.HostPort, _ready)
					if _ready {
						p.nodesOffline.Del(node.HostPort)
					}
					nodeProcessLock.Lock()
					delete(nodeProcess, node.HostPort)
					nodeProcessLock.Unlock()
				}
			}()
		}
		wg.Wait()
	}
}

func (p *Proxy) saveWriteData(q delayQueryChanStruct, wg *sync.WaitGroup, hostport string) {
	member := fmt.Sprintf("delayQuery_%s_%s", hostport, q.PreparedQueryKey)
	saveKey := fmt.Sprintf("delayQuery_%s", hostport)
	p.redisDB.Do(radix.Cmd(nil, "APPEND", member, string(q.Data)+"|"))
	p.redisDB.Do(radix.Cmd(nil, "ZADD", saveKey, q.QueryKey, member))
	p.redisDB.Do(radix.Cmd(nil, "SET", member+"_expired", "OK", "EX", "10"))
	wg.Done()
}

func (p *Proxy) delayQueryOpt() {
	var wg sync.WaitGroup
	for {
		select {
		case q := <-p.delayQueryChan:
			nodes := config.GetNodes()
			wg.Add(len(nodes))
			for _, n := range nodes {
				node := n
				go p.saveWriteData(q, &wg, node.HostPort)
			}
			wg.Wait()
		}
	}
}

func (p *Proxy) restoreNodeData(hostPort string, backend net.Conn) (err error) {
	p.restoreLock[hostPort].Lock()
	p.cleanRestoreData(hostPort)
	log.Debugf("restoreNodeData %v", hostPort)
	nodeKey := fmt.Sprintf("delayQuery_%s", hostPort)
	var finishItems []string
	err = p.redisDB.Do(radix.Cmd(&finishItems, "ZRANGE", nodeKey, "0", "-1"))
	if err != nil || len(finishItems) == 0 {
		log.Debugf("restoreNodeData Node:%v err:%v", hostPort, err)
		p.restoreLock[hostPort].Unlock()
		return
	}

	log.Infof("Node:'%s' Restoring... Query Length:%v", hostPort, len(finishItems))
	for _, v := range finishItems {
		var _queries string
		err = p.redisDB.Do(radix.Cmd(&_queries, "GET", v))
		if err != nil || _queries == "" {
			log.Errorf("cleanData Node:%v err:%v", hostPort, err)
			p.restoreLock[hostPort].Unlock()
			return
		}

		queries := strings.Split(_queries, "|")
		if len(queries) != 4 {
			log.Infof("Send Data:%v", _queries)
		}
		for _, q := range queries {
			if q == "" {
				continue
			}

			if _, err = connect.Send(backend, []byte(q)); err != nil {
				log.Errorf("Restore Node:'%s' Send Error: %s", hostPort, err.Error())
				p.restoreLock[hostPort].Unlock()
				return
			}

			go p.stats.Add(hostPort, common.StatsTypeOther)

			/*
			 * Continue to read from the backend until a 'ReadyForQuery' message is
			 * is found.
			 */
			buf := bytes.NewBuffer([]byte{})
			var messageType byte
			var message []byte
			var length int
			var done bool
			for !done {
				if message, length, err = connect.Receive(backend); err != nil {
					log.Errorf("Error receiving response from backend1 %s", backend.RemoteAddr())
					log.Errorf("Error: %s", err.Error())
					p.restoreLock[hostPort].Unlock()
					return
				}

				buf.Write(message[:length])
				log.Debugf("HandleWriteData message:%v length:%v", string(message[:length]), length)

				/*
				 * Examine all of the messages in the buffer and determine if any of
				 * them are a ReadyForQuery message.
				 */
				bufLen := buf.Len()
				for start := 0; start < bufLen; {
					messageType = protocol.GetMessageType(buf.Bytes()[start:])
					messageLength := int32(len(buf.Bytes()[start:]))
					if len(buf.Bytes()[start:]) >= 5 {
						messageLength = protocol.GetMessageLength(buf.Bytes()[start:])
					}
					log.Debugf("restoreNodeData start messageType %v messageLength:%v", string(messageType), messageLength)
					oldStart := start
					/*
					 * Calculate the next start position, add '1' to the message
					 * length to account for the message type.
					 */

					start = (start + int(messageLength) + 1)
					if start >= bufLen {
						log.Debugf("old %d start %d length %d", oldStart, start, bufLen)
						newBytes := buf.Bytes()[oldStart:]
						buf.Reset()
						buf.Write(newBytes)
						messageType = protocol.GetMessageType(buf.Bytes())
						break
					}
					log.Debugf("restoreNodeData messageType %v messageLength:%v %v", string(messageType), messageLength, string(buf.Bytes()[oldStart:start]))
				}
				log.Debugf("restoreNodeData message =========================================================================================== ")

				done = (messageType == protocol.ReadyForQueryMessageType)
			}
		}
		p.redisDB.Do(radix.Cmd(nil, "DEL", v, v+"_tmp"))
		p.redisDB.Do(radix.Cmd(nil, "ZREM", nodeKey, v))
	}
	var dataLen int
	p.redisDB.Do(radix.Cmd(&dataLen, "ZCARD", nodeKey))
	if dataLen > 0 {
		p.restoreLock[hostPort].Unlock()
		p.restoreNodeData(hostPort, backend)
		return
	}
	log.Infof("Node:'%s' Restored", hostPort)
	p.restoreLock[hostPort].Unlock()
	return
}
func (p *Proxy) cleanRestoreData(hostPort string) {
	p.cleanLock[hostPort].Lock()
	defer p.cleanLock[hostPort].Unlock()
	log.Debugf("cleanRestoreData start------------%v", hostPort)
	nodeKey := fmt.Sprintf("delayQuery_%s", hostPort)
	var finishItems []string
	err := p.redisDB.Do(radix.Cmd(&finishItems, "ZRANGE", nodeKey, "0", "-1"))
	if err != nil || len(finishItems) == 0 {
		log.Debugf("cleanRestoreData Node:%v err:%v", hostPort, err)
		return
	}
	var tempLen int
	var expiredVal string
	var _data string
	for _, v := range finishItems {
		p.redisDB.Do(radix.Cmd(&tempLen, "GET", v+"_tmp"))
		p.redisDB.Do(radix.Cmd(&expiredVal, "GET", v+"_expired"))
		p.redisDB.Do(radix.Cmd(&_data, "GET", v))
		_dataExplode := strings.Split(_data, "|")
		if tempLen == -(len(_dataExplode)-1) && expiredVal == "" {
			log.Debugf("cleanRestoreData tempLen:%v data:%s", tempLen, _data)
			p.redisDB.Do(radix.Cmd(nil, "DEL", v, v+"_tmp"))
			p.redisDB.Do(radix.Cmd(nil, "ZREM", nodeKey, v))
		}
	}
	log.Debugf("cleanRestoreData finish------------")
	return
}
