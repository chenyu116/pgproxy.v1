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
	"os"

	"github.com/spf13/cobra"
)

func Start() {
	if err := Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Failed running %q\n", os.Args[1])
		os.Exit(1)
	}
}

var pgProxyCmd = &cobra.Command{
	Use:          "pgproxy",
	Short:        "A Pool Proxy based PostgreSQL",
	SilenceUsage: true,
}

func init() {
	cobra.EnableCommandSorting = false

	pgProxyCmd.AddCommand(
		startCmd,
		stopCmd,
	)
}

func Run(args []string) error {
	pgProxyCmd.SetArgs(args)
	return pgProxyCmd.Execute()
}
