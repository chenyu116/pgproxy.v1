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

package config

import (
	"time"

	"github.com/spf13/viper"

	"github.com/jookco/pgproxy.v1/common"
	"github.com/jookco/pgproxy.v1/utils/log"
)

var c Config

func init() {
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
}

func GetConfig() Config {
	return c
}

func GetNodes() map[string]common.Node {
	return c.Nodes
}

func GetNodeByHostPort(hostPort string) (node common.Node) {
	for _, v := range c.Nodes {
		if v.HostPort == hostPort {
			node = v
			return
		}
	}
	return
}

func GetRedisConfig() common.RedisConfig {
	return c.Redis
}

func GetProxyConfig() ProxyConfig {
	return c.Server.Proxy
}

func GetApiConfig() ApiConfig {
	return c.Server.Api
}

func GetNodeWritePools() int {
	return c.WriteNode.WritePools
}

func GetWriteNodeConfig() WriteNodeConfig {
	return c.WriteNode
}

func GetCredentials() common.Credentials {
	return c.Credentials
}

func GetHealthCheckConfig() common.HealthCheckConfig {
	return c.HealthCheck
}

func Get(key string) interface{} {
	return viper.Get(key)
}

func GetBool(key string) bool {
	return viper.GetBool(key)
}

func GetInt(key string) int {
	return viper.GetInt(key)
}

func GetString(key string) string {
	return viper.GetString(key)
}

func GetStringMapString(key string) map[string]string {
	return viper.GetStringMapString(key)
}

func GetStringMap(key string) map[string]interface{} {
	return viper.GetStringMap(key)
}

func GetStringSlice(key string) []string {
	return viper.GetStringSlice(key)
}

func IsSet(key string) bool {
	return viper.IsSet(key)
}

func Set(key string, value interface{}) {
	viper.Set(key, value)
}

type ProxyConfig struct {
	HostPort string `mapstructure:"hostport"`
}

type ApiConfig struct {
	HostPort string `mapstructure:"hostport"`
}

type ServerConfig struct {
	Api   ApiConfig   `mapstructure:"api"`
	Proxy ProxyConfig `mapstructure:"proxy"`
}

type WriteNodeConfig struct {
	WritePools          int           `mapstructure:"writepools"`
	WritePoolsDelayTime time.Duration `mapstructure:"writepoolsdelaytime"`
}

type Config struct {
	Server      ServerConfig             `mapstructure:"server"`
	WriteNode   WriteNodeConfig          `mapstructure:"writenode"`
	Nodes       map[string]common.Node   `mapstructure:"nodes"`
	Credentials common.Credentials       `mapstructure:"credentials"`
	HealthCheck common.HealthCheckConfig `mapstructure:"healthcheck"`
	Redis       common.RedisConfig       `mapstructure:"redis"`
}

func SetConfigPath(path string) {
	viper.SetConfigFile(path)
}

func ReadConfig() {
	err := viper.ReadInConfig()
	log.Debugf("Using configuration file: %s", viper.ConfigFileUsed())

	if err != nil {
		log.Fatal(err.Error())
	}

	err = viper.Unmarshal(&c)

	if err != nil {
		log.Errorf("Error unmarshaling configuration file: %s", viper.ConfigFileUsed())
		log.Fatalf(err.Error())
	}
}
