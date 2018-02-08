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
	"net"
	"time"

	_ "github.com/lib/pq" // required
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/jookco/pgproxy.v1/server/apipb"
	"github.com/jookco/pgproxy.v1/utils/log"
)

type ApiServer struct {
	grpc   *grpc.Server
	server *Server
}

func NewApiServer(s *Server) *ApiServer {
	apiServer := &ApiServer{
		server: s,
	}

	apiServer.grpc = grpc.NewServer()

	pb.RegisterApiServer(apiServer.grpc, apiServer)

	return apiServer
}

func (s *ApiServer) Shutdown(context.Context, *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	// Stop the Proxy Server
	log.Debugf("Shutdown")
	s.server.proxy.Stop()

	// Stop the Admin grpc Server
	defer func() {
		time.Sleep(time.Second)
		s.grpc.Stop()
	}()

	return &pb.ShutdownResponse{}, nil
}

func (s *ApiServer) Serve(l net.Listener) {
	log.Infof("Api Server listening on: %s", l.Addr())
	defer s.server.waitGroup.Done()

	err := s.grpc.Serve(l)
	l.Close()
	if err != nil {
		log.Infof("Grpc Serve Error: %s", err.Error())
	}
}
