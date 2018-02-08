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

package common

import (
	"time"
)

const (
	NODE_ROLE_WRITER string = "writer"
	NODE_ROLE_READER string = "reader"
	_                       = iota
	StatsTypeSelect
	StatsTypeInsert
	StatsTypeUpdate
	StatsTypeAlter
	StatsTypeSetVAL
	StatsTypeOther
)

type Node struct {
	HostPort   string `mapstructure:"hostport"` //remote host:port
	Role       string `mapstructure:"role"`
	WritePools int    `mapstructure:"writepools"`
	ReadPools  int    `mapstructure:"readpools"`
}

type SSLConfig struct {
	Enable    bool   `mapstructure:"enable"`
	SSLMode   string `mapstructure:"sslmode,omitempty"`
	SSLCert   string `mapstructure:"sslcert,omitempty"`
	SSLKey    string `mapstructure:"sslkey,omitempty"`
	SSLRootCA string `mapstructure:"sslrootca,omitempty"`
}

type Credentials struct {
	Username string            `mapstructure:"username"`
	Password string            `mapstructure:"password,omitempty"`
	Database string            `mapstructure:"database"`
	SSL      SSLConfig         `mapstructure:"ssl"`
	Options  map[string]string `mapstructure:"options"`
}

type RedisConfig struct {
	HostPort string `mapstructure:"hostport"`
	Password string `mapstructure:"password,omitempty"`
	Database string `mapstructure:"database"`
}

type HealthCheckConfig struct {
	Delay           time.Duration `mapstructure:"delay"`
	ConnectTimeout  time.Duration `mapstructure:"connecttimeout"`
	QueryLogExpired string        `mapstructure:"querylogexpired"`
}
