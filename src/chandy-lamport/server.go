package chandy_lamport

import "log"

// The main participant of the distributed snapshot protocol.
// Servers exchange token messages and marker messages among each other.
// Token messages represent the transfer of tokens from one server to another.
// Marker messages represent the progress of the snapshot process. The bulk of
// the distributed protocol is implemented in `HandlePacket` and `StartSnapshot`.
type Server struct {
	Id            string
	Tokens        int
	sim           *Simulator
	outboundLinks map[string]*Link // key = link.dest
	inboundLinks  map[string]*Link // key = link.src
	// TODO: ADD MORE FIELDS HERE
	receivedMarkers map[int]map[string]bool
	activeSnapshots *SyncMap
	completedSnapshots *SyncMap
}


// A unidirectional communication channel between two servers
// Each link contains an event queue (as opposed to a packet queue)
type Link struct {
	src    string
	dest   string
	events *Queue
}

func NewServer(id string, tokens int, sim *Simulator) *Server {
	return &Server{
		id,
		tokens,
		sim,
		make(map[string]*Link),
		make(map[string]*Link),
		make(map[int]map[string]bool),
		NewSyncMap(),
		NewSyncMap(),
	}
}

// Add a unidirectional link to the destination server
func (server *Server) AddOutboundLink(dest *Server) {
	if server == dest {
		return
	}
	l := Link{server.Id, dest.Id, NewQueue()}
	server.outboundLinks[dest.Id] = &l
	dest.inboundLinks[server.Id] = &l
}

// Send a message on all of the server's outbound links
func (server *Server) SendToNeighbors(message interface{}) {
	for _, serverId := range getSortedKeys(server.outboundLinks) {
		link := server.outboundLinks[serverId]
		server.sim.logger.RecordEvent(
			server,
			SentMessageEvent{server.Id, link.dest, message})
		link.events.Push(SendMessageEvent{
			server.Id,
			link.dest,
			message,
			server.sim.GetReceiveTime()})
	}
}

// Send a number of tokens to a neighbor attached to this server
func (server *Server) SendTokens(numTokens int, dest string) {
	if server.Tokens < numTokens {
		log.Fatalf("Server %v attempted to send %v tokens when it only has %v\n",
			server.Id, numTokens, server.Tokens)
	}
	message := TokenMessage{numTokens}
	server.sim.logger.RecordEvent(server, SentMessageEvent{server.Id, dest, message})
	// Update local state before sending the tokens
	server.Tokens -= numTokens
	link, ok := server.outboundLinks[dest]
	if !ok {
		log.Fatalf("Unknown dest ID %v from server %v\n", dest, server.Id)
	}
	link.events.Push(SendMessageEvent{
		server.Id,
		dest,
		message,
		server.sim.GetReceiveTime()})
}

// Callback for when a message is received on this server.
// When the snapshot algorithm completes on this server, this function
// should notify the simulator by calling `sim.NotifySnapshotComplete`.
func (server *Server) HandlePacket(src string, message interface{}) {
	// TODO: IMPLEMENT ME
	server.activeSnapshots.Range(func(k, v interface{}) bool {
		if snapshot,found := v.(*SnapshotState); found{
			if _, ok := server.receivedMarkers[snapshot.id][src]; !ok {
			switch msg := message.(type) {
			case MarkerMessage:
				return true
			case TokenMessage:
				snapshot.messages = append(snapshot.messages, &SnapshotMessage{src, server.Id, msg})
			}
		}
	}
		return true
	})
	switch msg := message.(type) {
	case TokenMessage:
		server.Tokens += msg.numTokens
	case MarkerMessage:
		msgId := msg.snapshotId
		_, ok := server.activeSnapshots.Load(msgId)
		if !ok {
			server.StartSnapshot(msgId)
		}
		
		mrkd := server.receivedMarkers[msgId]
		mrkd[src] = true
		
		if len(mrkd) == len(server.inboundLinks) {
			snp,_ := server.activeSnapshots.Load(msgId)
			server.completedSnapshots.Store(msgId,snp)
			server.sim.NotifySnapshotComplete(server.Id, msgId)
		}
	default:
		 log.Fatal("Unknown message type for message ", msg)
	}
}

// Start the chandy-lamport snapshot algorithm on this server.
// This should be called only once per server.
func (server *Server) StartSnapshot(snapshotId int) {
	// TODO: IMPLEMENT ME
	initialToks := make(map[string]int)
	initialToks[server.Id] = server.Tokens
	newSnapshot := SnapshotState{snapshotId, initialToks, make([]*SnapshotMessage,0)}
	server.activeSnapshots.LoadOrStore(snapshotId, &newSnapshot) 
	server.receivedMarkers[snapshotId] = make(map[string]bool)
	server.SendToNeighbors(MarkerMessage{snapshotId})
}
