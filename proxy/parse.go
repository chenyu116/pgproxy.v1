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
	"strings"

	"github.com/jookco/pgproxy.v1/protocol"
	"github.com/jookco/pgproxy.v1/utils/log"
)

var queryWrite []string = []string{"select setval", "insert", "update", "delete", "copy", "create", "alter", "drop"}

// GetQueryIsRead the query if select or set
// return booleans
func getQueryIsRead(m []byte) bool {
	messageType := protocol.GetMessageType(m)
	messageLength := protocol.GetMessageLength(m)
	if messageLength == 5 {
		return true
	}
	startIndex := 5
	switch messageType {
	case protocol.ParseMessageType:
		startIndex = 22
	}
	endIndex := startIndex + 15
	msgString := strings.ToLower(string(m[startIndex:endIndex]))
	for _, v := range queryWrite {
		if strings.Contains(msgString, v) {
			return false
		}
	}
	log.Debugf("getQueryIsRead msg:%v %v", string(m), messageType)
	return true
}

// getQueryIsTransaction the query if transaction
// return booleans
func getQueryIsTransaction(m []byte) bool {
	msgString := strings.ToLower(string(m[5:10]))
	log.Debugf("getQueryIsTransaction %v", msgString)
	if strings.Contains(msgString, "begin") {
		return true
	}
	return false
}

// getQueryIsTransactionFinish the query if transaction is finish
// return booleans
func getQueryIsTransactionFinish(m []byte) bool {
	msgString := strings.ToLower(string(m[5:15]))
	log.Debugf("getQueryIsTransactionFinish %v", msgString)
	if strings.Contains(msgString, "commit") || strings.Contains(msgString, "rollback") {
		return true
	}
	return false
}
