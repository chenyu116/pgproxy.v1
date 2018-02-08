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

package cli

import (
	"fmt"

	pb "github.com/jookco/pgproxy.v1/server/apipb"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop a running instance of a proxy",
	RunE:  runStop,
}

func init() {
	flags := stopCmd.Flags()

	stringFlag(flags, &host, FlagApiHost)
	stringFlag(flags, &port, FlagApiPort)
}

func runStop(cmd *cobra.Command, args []string) error {
	fmt.Println("PgProxy Server Exiting...")
	address := fmt.Sprintf("%s:%s", host, port)

	dialOptions := []grpc.DialOption{
		grpc.WithDialer(adminServerDialer),
		grpc.WithInsecure(),
	}

	conn, err := grpc.Dial(address, dialOptions...)
	if err != nil {
		fmt.Println("No Proxy Server Alive on: ", address)
		return nil
	}
	defer conn.Close()
	c := pb.NewApiClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Shutdown(ctx, &pb.ShutdownRequest{})
	fmt.Println("PgProxy Server Exited!")

	return nil
}
