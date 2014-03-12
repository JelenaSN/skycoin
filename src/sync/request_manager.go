package sync

import (
	//"crypto/sha256"
	//"hash"
	//"errors"
	//"github.com/skycoin/gnet"
	"fmt"
	"log"
	"time"
)

/*
	Request mangager handles rate limiting data requests on a per peer basis
*/
/*
	Todo:
	- split hash lists into multiple pages
	- do query for each page from remote peer
	-

*/

//Todo: Anti DDoS
// - limit requests on per peer basis
// - determine which peers are giving us data
// - kick peers

//open request
type Request struct {
	RequestTime int    //time of request
	Addr        string //address request was made to
}

type PeerStats struct {
	Addr             string
	OpenRequests     int
	LastRequest      int //time last request was received
	FinishedRequests int //number of requests served

	Data map[SHA256]int //hash to time received

}

type RequestManagerConfig struct {
	RequestTimeout  int //timeout for requests
	RequestsPerPeer int //max requests per peer
}

func NewRequestManagerConfig() RequestManagerConfig {
	return RequestManagerConfig{
		RequestTimeout:  20,
		RequestsPerPeer: 6,
	}
}

//this makes the request
type RequestFunction func(hash SHA256, addr string)

type RequestManager struct {
	Config RequestManagerConfig

	PeerStats map[string]*PeerStats
	Requests  map[SHA256]Request //hash to time

	RequestFunction RequestFunction
}

func NewRequestManager(config RequestManagerConfig, requestFunction RequestFunction) RequestManager {
	var rm RequestManager
	//rm.Requests = make(map[SHA256]Request)
	//rm.Data = make(map[SHA256][]string)
	rm.Config = config
	rm.RequestFunction = requestFunction

	return rm
}

//send out requests and clears timeouts
func (self *RequestManager) Tick() {
	self.removeExpiredRequests()
	self.tickRequests()
}

func (self *RequestManager) removeExpiredRequests() {
	t := int(time.Now().Unix())
	for k, r := range self.Requests {
		if t-r.RequestTime < self.Config.RequestTimeout {
			delete(self.Requests, k)
		}
	}
}

//physically make request for data, by hash
func (self *RequestManager) makeRequest(hash SHA256, addr string) {
	fmt.Printf("addr: %s request: %s \n", addr, hash.Hex())

	//add request to list
	req := Request{
		RequestTime: int(time.Now().Unix()),
		Addr:        addr,
	}
	//self.Requests = append(self.Requests, req)
	self.Requests[hash] = req

	//increment open requests for peer
	self.PeerStats[addr].OpenRequests += 1
	self.RequestFunction(hash, addr) //call external request function
}

//call when there is new data to download
func (self *RequestManager) DataAnnounce(hashList []SHA256, addr string) {
	t := int(time.Now().Unix())
	for _, hash := range hashList {
		self.PeerStats[addr].Data[hash] = t
	}
}

//call when request FinishedRequests
func (self *RequestManager) RequestFinished(hash SHA256, addr string) {
	//remove data from peer data list
	if _, ok := self.PeerStats[addr].Data[hash]; ok == false {
		log.Printf("RequestFinished: warning received unannounced data from peer, addr= %s, hash= %s \n", addr, hash.Hex())
		return
	} else {
		delete(self.PeerStats[addr].Data, hash)
	}

	if _, ok := self.Requests[hash]; ok == false {
		log.Printf("RequestFinished: warning received unrequested data from peer, addr= %s, hash= %s \n", addr, hash.Hex())
	} else {
		delete(self.Requests, hash)
	}

	self.PeerStats[addr].OpenRequests -= 1
	self.PeerStats[addr].FinishedRequests += 1
	self.PeerStats[addr].LastRequest = int(time.Now().Unix())
}

//current implementation requests data in random order
func (self *RequestManager) tickRequests() {
	for addr, p := range self.PeerStats {

		if p.OpenRequests < self.Config.RequestsPerPeer {
			var hash SHA256
			for h, _ := range p.Data {
				if _, ok := self.Requests[h]; ok == false {
					self.makeRequest(hash, addr)
					break
				}
			}
		}
	}
}

//called when peer connects
func (self *RequestManager) OnConnect(addr string) {

	self.PeerStats[addr] = &PeerStats{
		Addr: addr,
	}
}

//called when peer disconnects
func (self *RequestManager) OnDisconnect(addr string) {

	delete(self.PeerStats, addr)

	for _, r := range self.Requests {
		if r.Addr == addr {
			r.RequestTime = 0 //set request for collection
		}
	}
}
