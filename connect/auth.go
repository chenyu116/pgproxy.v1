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

package connect

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/jookco/pgproxy.v1/config"
	"github.com/jookco/pgproxy.v1/protocol"
	"github.com/jookco/pgproxy.v1/utils/log"
)

/*
 * Handle authentication requests that are sent by the backend to the client.
 *
 * connection - the connection to authenticate against.
 * message - the authentication message sent by the backend.
 */
func HandleAuthenticationRequest(connection net.Conn, message []byte) bool {
	var authType int32

	reader := bytes.NewReader(message[5:9])
	binary.Read(reader, binary.BigEndian, &authType)

	switch authType {
	case protocol.AuthenticationKerberosV5:
		log.Errorf("KerberosV5 authentication is not currently supported.")
	case protocol.AuthenticationClearText:
		log.Debugf("Authenticating with clear text password.")
		return handleAuthClearText(connection)
	case protocol.AuthenticationMD5:
		log.Debugf("Authenticating with MD5 password.")
		return handleAuthMD5(connection, message)
	case protocol.AuthenticationSCM:
		log.Errorf("SCM authentication is not currently supported.")
	case protocol.AuthenticationGSS:
		log.Errorf("GSS authentication is not currently supported.")
	case protocol.AuthenticationGSSContinue:
		log.Errorf("GSS authentication is not currently supported.")
	case protocol.AuthenticationSSPI:
		log.Errorf("SSPI authentication is not currently supported.")
	case protocol.AuthenticationOk:
		/* Covers the case where the authentication type is 'cert' or 'trust' */
		return true
	default:
		log.Errorf("Unknown authentication method: %d", authType)
	}

	return false
}

func createMD5Password(username string, password string, salt string) string {
	// Concatenate the password and the username together.
	passwordString := fmt.Sprintf("%s%s", password, username)

	// Compute the MD5 sum of the password+username string.
	passwordString = fmt.Sprintf("%x", md5.Sum([]byte(passwordString)))

	// Compute the MD5 sum of the password hash and the salt
	passwordString = fmt.Sprintf("%s%s", passwordString, salt)
	return fmt.Sprintf("md5%x", md5.Sum([]byte(passwordString)))
}

func handleAuthMD5(connection net.Conn, message []byte) bool {
	// Get the authentication credentials.
	creds := config.GetCredentials()
	username := creds.Username
	password := creds.Password
	salt := string(message[9:13])

	password = createMD5Password(username, password, salt)

	// Create the password message.
	passwordMessage := protocol.CreatePasswordMessage(password)

	// Send the password message to the backend.
	_, err := Send(connection, passwordMessage)

	// Check that write was successful.
	if err != nil {
		log.Error("Error sending password message to the backend.")
		log.Errorf("Error: %s", err.Error())
	}

	// Read response from password message.
	message, _, err = Receive(connection)

	// Check that read was successful.
	if err != nil {
		log.Error("Error receiving authentication response from the backend.")
		log.Errorf("Error: %s", err.Error())
	}

	return protocol.IsAuthenticationOk(message)
}

func handleAuthClearText(connection net.Conn) bool {
	password := config.GetString("credentials.password")
	passwordMessage := protocol.CreatePasswordMessage(password)

	_, err := connection.Write(passwordMessage)

	if err != nil {
		log.Error("Error sending clear text password message to the backend.")
		log.Errorf("Error: %s", err.Error())
	}

	response := make([]byte, 4096)
	_, err = connection.Read(response)

	if err != nil {
		log.Error("Error receiving clear text authentication response.")
		log.Errorf("Error: %s", err.Error())
	}

	return protocol.IsAuthenticationOk(response)
}

// AuthenticateClient - Authenticate client connection to the backend.
func AuthenticateClient(client net.Conn, message []byte, length int, hostPort string) (bool, error) {
	var err error

	// /* Establish a connection with the backend node. */
	log.Debugf("client auth: connecting to 'backend' node")
	backend, err := Connect(hostPort)

	if err != nil {
		log.Errorf("An error occurred connecting to the backend node")
		log.Errorf("Error %s", err.Error())
		return false, err
	}

	defer backend.Close()
	/* Relay the startup message to backend node. */
	log.Debug("client auth: relay startup message to 'backend' node")
	_, err = backend.Write(message[:length])
	if err != nil {
		log.Error("An error occurred connecting to the backend node")
		log.Errorf("Error %s", err.Error())
		return false, err
	}

	/* Receive startup response. */
	log.Debug("client auth: receiving startup response from 'backend' node")
	message, length, err = Receive(backend)

	if err != nil {
		log.Error("An error occurred receiving startup response.")
		log.Errorf("Error %s", err.Error())
		return false, err
	}

	/*
	 * While the response for the backend node is not an AuthenticationOK or
	 * ErrorResponse keep relaying the mesages to/from the client/backend.
	 */
	messageType := protocol.GetMessageType(message)

	for !protocol.IsAuthenticationOk(message) &&
		(messageType != protocol.ErrorMessageType) {
		Send(client, message[:length])
		message, length, err = Receive(client)

		/*
		 * Must check that the client has not closed the connection.
		 */
		if (err != nil) && (err == io.EOF) {
			log.Info("The client closed the connection.")
			log.Debug("If the client is 'psql' and the authentication method " +
				"was 'password', then this behavior is expected.")
			return false, err
		}

		Send(backend, message[:length])

		message, length, err = Receive(backend)

		messageType = protocol.GetMessageType(message)
	}

	/*
	 * If the last response from the backend node was AuthenticationOK, then
	 * terminate the connection and return 'true' for a successful
	 * authentication of the client.
	 */
	log.Debug("client auth: checking authentication repsonse")
	if protocol.IsAuthenticationOk(message) {
		termMsg := protocol.GetTerminateMessage()
		Send(backend, termMsg)
		Send(client, message[:length])
		return true, nil
	}

	if protocol.GetMessageType(message) == protocol.ErrorMessageType {
		err = protocol.ParseError(message)
		log.Error("Error occurred on client startup.")
		log.Errorf("Error: %s", err.Error())
	} else {
		log.Error("Unknown error occurred on client startup.")
	}

	Send(client, message[:length])

	return false, err
}

func ValidateClient(message []byte) bool {
	var clientUser string
	var clientDatabase string

	creds := config.GetCredentials()

	buf := protocol.NewMessageBuffer(message)

	buf.Seek(8) // Seek past the message length and protocol version.

	for {
		param, err := buf.ReadString()

		if err == io.EOF || param == "\x00" {
			break
		}

		switch param {
		case "user":
			clientUser, err = buf.ReadString()
		case "database":
			clientDatabase, err = buf.ReadString()
		}
	}
	return (clientUser == creds.Username && clientDatabase == creds.Database)
}
