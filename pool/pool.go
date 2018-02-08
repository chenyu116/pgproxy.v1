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

package pool

import (
	"net"
)

type Pool struct {
	connections chan net.Conn
	Name        string
	Ready       bool
}

func NewPool(name string, capacity int) *Pool {
	return &Pool{
		connections: make(chan net.Conn, capacity),
		Name:        name,
	}
}

func (p *Pool) Add(connection net.Conn) {
	p.connections <- connection
}

func (p *Pool) Next() net.Conn {
	return <-p.connections
}

func (p *Pool) NextNil() net.Conn {
	for {
		select {
		case conn := <-p.connections:
			return conn
		default:
			return nil
		}
	}
}

func (p *Pool) Return(connection net.Conn) {
	p.connections <- connection
}

func (p *Pool) Len() int {
	return len(p.connections)
}

func (p *Pool) Close() {
	for {
		select {
		case conn := <-p.connections:
			conn.Close()
		default:
			return
		}
	}
}
