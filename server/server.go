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

package server

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/jookco/pgproxy.v1/config"
	"github.com/jookco/pgproxy.v1/utils/log"
	"github.com/mediocregopher/radix.v3"
)

type Server struct {
	api       *ApiServer
	proxy     *ProxyServer
	waitGroup *sync.WaitGroup
	redisDb   *radix.Pool
}

func NewServer() *Server {
	s := &Server{
		waitGroup: &sync.WaitGroup{},
	}

	s.proxy = NewProxyServer(s)

	s.api = NewApiServer(s)
	return s
}

func (s *Server) Start() {
	log.Infof("Proxy Server Starting...")
	proxyConfig := config.GetProxyConfig()
	proxyListener, err := net.Listen("tcp", proxyConfig.HostPort)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	s.waitGroup.Add(1)
	go s.proxy.Serve(proxyListener)

	log.Infof("Api Server Starting...")
	apiConfig := config.GetApiConfig()
	apiListener, err := net.Listen("tcp", apiConfig.HostPort)

	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	s.waitGroup.Add(1)
	go s.api.Serve(apiListener)

	s.waitGroup.Wait()

	fmt.Println("PgProxy Server Stopped!")
}
