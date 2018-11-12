package status

import (
	"sync"
	"time"
)

type BlockInfo struct {
	Height int		`json:"height"`
	Id string		`json:"id"`
	Time time.Time	`json:"time"`
}

type ChainId int
type NetType int
type blockMap map[ChainId]map[NetType]BlockInfo

type BlockchainInfo struct {
	ChainId ChainId				`json:"chain_id"`
	NetType NetType				`json:"net_type"`
	LastSeenBlock BlockInfo		`json:"last_seen_block"`
}

type ClientRequestsInfo struct {
	TotalHandled uint			`json:"total_handled"`
	LastRequestTime time.Time	`json:"last_time"`
}

type DatabaseInfo struct {
	Status string				`json:"status"`
	TotalQueriesRan uint			`json:"queries_ran"`
	LastQueryTime time.Time		`json:"query_last_time"`
}

type Status struct {
	StartTime time.Time						`json:"start_time"`
	DatabaseInfo DatabaseInfo				`json:"database"`
	ClientRequestsInfo ClientRequestsInfo	`json:"requests"`
	AllBlockchainInfo []BlockchainInfo		`json:"blockchains"`
}

type BlockType int

type UpdateCallback func()
type DatabaseQueriesCollector func(newQueries uint)
type ClientRequestsCollector func(newRequests uint)
type BlockchainInfoCollector func(blockinfo BlockInfo)

// Collects all status information and provides that upon request.
type Collector struct {
	sync.Mutex

	startTime time.Time
	databaseInfo DatabaseInfo
	clientRequestsInfo ClientRequestsInfo
	blockMap blockMap
	updateCallback UpdateCallback
}

func NewCollector() *Collector {
	return &Collector{
		blockMap : make(blockMap),
		startTime: time.Now(),
	}
}

func NewCollectorWithCallback(updateCallback UpdateCallback) (result *Collector) {
	result = NewCollector()
	result.updateCallback = updateCallback

	return
}

func (self *Collector) NewDatabaseQueryCollector() DatabaseQueriesCollector {
	return func(newQueries uint) {
		self.onDatabaseQueries(newQueries)
	}
}

func (self *Collector) NewClientRequestsCollector() ClientRequestsCollector {
	return func(newRequests uint) {
		self.onClientRequests(newRequests)
	}
}

func (self *Collector) NewBlockchainInfoCollector(chainId int, netType int) BlockchainInfoCollector {
	return func(blockinfo BlockInfo) {
		self.onBlockchainInfo(chainId, netType, blockinfo)
	}
}

func (self *Collector) GetStatus() Status {
	// estimation is that we have 2 netTypes and 2 blockInfos for each chain:
	blockchainsInfo := make([]BlockchainInfo, len(self.blockMap) * 4)

	for chainId, netMap := range self.blockMap {
		for netType, block := range netMap {
			blockchainsInfo = append(blockchainsInfo,
				BlockchainInfo{chainId, netType, block})
		}
	}

	return Status {
		self.startTime,
		self.databaseInfo,
		self.clientRequestsInfo,
		blockchainsInfo,
	}
}

func (self *Collector) SetStartTime(startTime time.Time) {
	self.startTime = startTime
}

func (self *Collector) OnDatabaseStatus(databaseStatus string) {
	self.Lock()
	defer func() {
		self.Unlock()
		self.onUpdate()
	}()

	self.databaseInfo.Status = databaseStatus
}

func (self *Collector) onDatabaseQueries(delta uint) {
	self.Lock()
	defer func() {
		self.Unlock()
		self.onUpdate()
	}()

	self.databaseInfo.TotalQueriesRan += delta
	self.databaseInfo.LastQueryTime = time.Now()
}

func (self *Collector) onClientRequests(delta uint) {
	self.Lock()
	defer func() {
		self.Unlock()
		self.onUpdate()
	}()

	self.clientRequestsInfo.TotalHandled += delta
	self.clientRequestsInfo.LastRequestTime = time.Now()
}

func (self *Collector) onBlockchainInfo(chainId int, netType int, blockInfo BlockInfo) {
	self.Lock()
	defer func() {
		self.Unlock()
		self.onUpdate()
	}()

	netMap, set := self.blockMap[ChainId(chainId)]
	if  set == false {
		netMap = make(map[NetType]BlockInfo)
		self.blockMap[ChainId(chainId)] = netMap
	}

	netMap[NetType(netType)] = blockInfo
}

func (self* Collector) onUpdate() {
	if self.updateCallback != nil {
		self.updateCallback()
	}
}
