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
	"sync"

	"github.com/jookco/pgproxy.v1/common"
	"github.com/jookco/pgproxy.v1/pool"
)

type writeNodeMap struct {
	Map map[string]*pool.Pool
	sync.RWMutex
}

func newWriteNodeMap() *writeNodeMap {
	return &writeNodeMap{Map: make(map[string]*pool.Pool)}
}

func (w *writeNodeMap) Get(hostPort string) *pool.Pool {
	w.RLock()
	defer w.RUnlock()
	if _pool, ok := w.Map[hostPort]; ok {
		return _pool
	}
	return nil
}

func (w *writeNodeMap) Set(hostPort string, writePool *pool.Pool) {
	w.Lock()
	defer w.Unlock()
	w.Map[hostPort] = writePool
}

func (w *writeNodeMap) Del(hostPort string) {
	w.Lock()
	defer w.Unlock()
	delete(w.Map, hostPort)
}

func newNodeReadyMap() *nodeReadyMap {
	return &nodeReadyMap{Map: make(map[string]bool)}
}

type nodeReadyMap struct {
	Map map[string]bool
	sync.RWMutex
}

func (n *nodeReadyMap) Get(hostPort string) bool {
	n.RLock()
	if s, ok := n.Map[hostPort]; ok {
		n.RUnlock()
		return s
	}
	n.RUnlock()
	return false
}

func (n *nodeReadyMap) GetOK() string {
	n.RLock()
	for hostPort, s := range n.Map {
		if s {
			n.RUnlock()
			return hostPort
		}
	}
	n.RUnlock()
	return ""
}

func (n *nodeReadyMap) Set(hostPort string, s bool) {
	n.Lock()
	defer n.Unlock()
	n.Map[hostPort] = s
}

func newNodeStatsMap() *nodeStatsMap {
	return &nodeStatsMap{Map: make(map[string]*ProxyStats)}
}

type nodeStatsMap struct {
	Map map[string]*ProxyStats
	sync.RWMutex
}

func (n *nodeStatsMap) Get(hostPort string) *ProxyStats {
	n.RLock()
	defer n.RUnlock()
	if _stats, ok := n.Map[hostPort]; ok {
		return _stats
	}
	return nil
}

func (n *nodeStatsMap) Add(hostPort string, stateType int) {
	n.Lock()
	defer n.Unlock()
	n.Map[hostPort].QueryCount++
	if stateType == common.StatsTypeSelect {
		n.Map[hostPort].SelectCount++
	} else {
		n.Map[hostPort].OtherCount++
	}
}

type offlineNodeMap struct {
	Map map[string]common.Node
	sync.RWMutex
}

func newOfflineNodeMap(nodesLen int) *offlineNodeMap {
	return &offlineNodeMap{Map: make(map[string]common.Node, nodesLen)}
}

func (o *offlineNodeMap) Set(node common.Node) {
	o.Lock()
	o.Map[node.HostPort] = node
	o.Unlock()
}
func (o *offlineNodeMap) Has(hostPort string) bool {
	o.RLock()
	defer o.RUnlock()
	if _, ok := o.Map[hostPort]; ok {
		return true
	}
	return false
}
func (o *offlineNodeMap) Del(hostPort string) {
	o.Lock()
	if _, ok := o.Map[hostPort]; ok {
		delete(o.Map, hostPort)
	}
	o.Unlock()
}
