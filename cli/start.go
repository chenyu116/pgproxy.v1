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
	"os"
	"os/exec"
	"strings"

	"github.com/jookco/pgproxy.v1/config"
	"github.com/jookco/pgproxy.v1/server"
	"github.com/jookco/pgproxy.v1/utils/log"
	"github.com/spf13/cobra"
)

var background bool
var configPath string
var logLevel string

var startCmd = &cobra.Command{
	Use:     "start",
	Short:   "start a PgProxy instance",
	Long:    "",
	Example: "",
	RunE:    runStart,
}

func init() {
	flags := startCmd.Flags()
	boolFlag(flags, &background, FlagBackground)
	stringFlag(flags, &configPath, FlagConfigPath)
	stringFlag(flags, &logLevel, FlagLogLevel)
}

func runStart(cmd *cobra.Command, args []string) error {
	if background {
		args = make([]string, 0, len(os.Args))

		for _, arg := range os.Args {
			if strings.HasPrefix(arg, "--background") || strings.HasPrefix(arg, "-d") {
				continue
			}
			args = append(args, arg)
		}

		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Start()
	}
	log.SetLevel(logLevel)

	if configPath != "" {
		config.SetConfigPath(configPath)
	}

	config.ReadConfig()
	s := server.NewServer()

	s.Start()

	return nil
}
