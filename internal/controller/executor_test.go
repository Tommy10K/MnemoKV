package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/resp"
	"github.com/mnemokv/mnemokv/internal/server"
)

func TestExecutorWithRealManagersRestoresWritesAndReplicaData(t *testing.T) {
	nodes, cfg := startManagerExecutorCluster(t)
	metadata := nodes[0].manager.Metadata()
	slot := uint32(0)
	before, _ := metadata.Slot(slot)
	if before.LeaderID != "node-1" || before.ReplicaID != "node-2" {
		t.Fatalf("unexpected initial slot: %+v", before)
	}
	key := keyForExactSlot(t, metadata, slot)
	frame := nodes[0].manager.Coordinator().Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte(key), []byte("before")}})
	if frame != resp.OK {
		t.Fatalf("initial write: %#v", frame)
	}

	nodes[0].stopData()
	nodes[0].reachable = false
	view := viewFromManagerMetadata(nodes[1].manager.Metadata().Snapshot(), "node-1")
	plan, ok := PlanFailover(view)
	if !ok {
		t.Fatal("expected manager-backed failover plan")
	}
	proposer := newFSMProposer(t, view, plan)
	clients := make(map[string]AdminNodeAPI, len(nodes))
	for _, node := range nodes {
		client, err := NewAuthenticatedNodeClient(node.http.URL, time.Second, "secret")
		if err != nil {
			t.Fatal(err)
		}
		clients[node.id] = client
	}
	executorCfg := cfg
	executorCfg.Controller = config.ControllerConfig{ObserveIntervalMs: 5, FailureTimeoutMs: 1000, MigrationRateLimit: 1000}
	if err := NewExecutor(executorCfg, clients, proposer).ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	repaired, _ := nodes[1].manager.Metadata().Slot(slot)
	if repaired.LeaderID != "node-2" || repaired.ReplicaID != "node-3" || !repaired.ReplicaReady {
		t.Fatalf("unexpected repaired slot: %+v", repaired)
	}
	for _, node := range nodes[1:] {
		entry, exists := node.engine.Store().Peek([]byte(key))
		if !exists || string(entry.Value.(*engine.StringValue).Data) != "before" {
			t.Fatalf("%s missing synchronized slot data", node.id)
		}
	}
	frame = nodes[1].manager.Coordinator().Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte(key), []byte("after")}})
	if frame != resp.OK {
		t.Fatalf("write did not resume after sync: %#v", frame)
	}
}

func TestExecutorCompletesFailoverAndRestoresTwoCopies(t *testing.T) {
	view, cluster, clients := executorScenario()
	plan, ok := PlanFailover(view)
	if !ok {
		t.Fatal("expected failover plan")
	}
	proposer := newFSMProposer(t, view, plan)
	executor := testExecutor(proposer, clients)
	if err := executor.ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := proposer.State()
	if state.ActivePlan != nil || state.LastCompletedPlanID != plan.ID {
		t.Fatalf("plan did not complete: %+v", state)
	}
	for _, slot := range cluster.snapshot().Slots {
		if slot.LeaderID == "node-1" || slot.ReplicaID == "node-1" || slot.LeaderID == slot.ReplicaID || !slot.ReplicaReady {
			t.Fatalf("slot was not restored to two surviving copies: %+v", slot)
		}
	}
	if cluster.callCount(StepPromote) != 1 || cluster.callCount(StepAssignReplica) != 2 || cluster.callCount(StepSync) != 2 {
		t.Fatalf("unexpected operation counts: %+v", cluster.calls)
	}
}

func TestExecutorResumesAfterLeadershipChangeWithoutDuplicates(t *testing.T) {
	view, cluster, clients := executorScenario()
	plan, _ := PlanFailover(view)
	proposer := newFSMProposer(t, view, plan)
	proposer.afterPropose = func(command Command) {
		if command.Type == CommandStepDone && cluster.callCount(StepPromote) == 1 {
			proposer.leader = false
		}
	}
	executor := testExecutor(proposer, clients)
	if err := executor.ExecuteOnce(context.Background()); !errors.Is(err, raft.ErrNotLeader) {
		t.Fatalf("leadership-loss error = %v", err)
	}
	proposer.mu.Lock()
	proposer.leader = true
	proposer.afterPropose = nil
	proposer.mu.Unlock()
	if err := executor.ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cluster.callCount(StepPromote) != 1 {
		t.Fatalf("promotion was duplicated %d times", cluster.callCount(StepPromote))
	}
}

func TestExecutorRetriesAfterTransientServerFailure(t *testing.T) {
	view, cluster, clients := executorScenario()
	cluster.failNext = map[StepKind]int{StepPromote: 1}
	plan, _ := PlanFailover(view)
	proposer := newFSMProposer(t, view, plan)
	executor := testExecutor(proposer, clients)
	if err := executor.ExecuteOnce(context.Background()); err == nil {
		t.Fatal("expected transient promotion failure")
	}
	if proposer.State().ActivePlan == nil {
		t.Fatal("transient failure discarded the active plan")
	}
	if err := executor.ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if proposer.State().ActivePlan != nil {
		t.Fatal("executor did not resume the plan")
	}
}

func TestExecutorMarksNoSurvivingCopyWithoutOwnershipCall(t *testing.T) {
	view := plannerView(
		[]SlotView{{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true}},
		failedNode("node-1"), failedNode("node-2"), eligibleNode("node-3", 0, 0),
	)
	plan, _ := PlanFailover(view)
	cluster := newFakeAdminCluster([]SlotStatus{{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true}})
	clients := cluster.clients("node-1", "node-2", "node-3")
	proposer := newFSMProposer(t, view, plan)
	if err := testExecutor(proposer, clients).ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := proposer.State()
	if _, ok := state.Unavailable[0]; !ok {
		t.Fatal("unrecoverable slot was not committed unavailable")
	}
	if len(cluster.calls) != 0 {
		t.Fatalf("unrecoverable slot triggered ownership calls: %+v", cluster.calls)
	}
}

func TestNodeClientSignsControlRequests(t *testing.T) {
	secret := "controller-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		canonical, _ := json.Marshal(struct {
			Slot uint32 `json:"slot"`
		}{7})
		index := r.Header.Get(ControlIndexHeader)
		want := signControlRequest([]byte(secret), http.MethodPost, "/cluster/promote", canonical, index)
		if index != "42" || r.Header.Get(ControlSignatureHeader) != want {
			t.Errorf("invalid fencing headers: index=%q signature=%q", index, r.Header.Get(ControlSignatureHeader))
		}
		_ = json.NewEncoder(w).Encode(ClusterAdminResponse{MetadataVersion: 2, Slot: SlotStatus{Number: 7}})
	}))
	defer server.Close()
	client, err := NewAuthenticatedNodeClient(server.URL, time.Second, secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Promote(context.Background(), 7, 42); err != nil {
		t.Fatal(err)
	}
}

func executorScenario() (ClusterView, *fakeAdminCluster, map[string]AdminNodeAPI) {
	slots := []SlotStatus{
		{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true},
		{Number: 1, LeaderID: "node-2", ReplicaID: "node-1", Term: 1, ReplicaReady: true},
	}
	view := plannerView(
		[]SlotView{
			{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true},
			{Number: 1, LeaderID: "node-2", ReplicaID: "node-1", Term: 1, ReplicaReady: true},
		},
		failedNode("node-1"), eligibleNode("node-2", 1, 1), eligibleNode("node-3", 0, 0),
	)
	cluster := newFakeAdminCluster(slots)
	return view, cluster, cluster.clients("node-1", "node-2", "node-3")
}

func testExecutor(proposer *fsmProposer, clients map[string]AdminNodeAPI) *Executor {
	cfg := config.ClusterConfig{Controller: config.ControllerConfig{ObserveIntervalMs: 1, FailureTimeoutMs: 50, MigrationRateLimit: 1000}}
	return NewExecutor(cfg, clients, proposer)
}

type fsmProposer struct {
	mu           sync.Mutex
	leader       bool
	index        uint64
	fsm          *FSM
	afterPropose func(Command)
}

func newFSMProposer(t *testing.T, view ClusterView, plan RecoveryPlan) *fsmProposer {
	t.Helper()
	p := &fsmProposer{leader: true, fsm: NewFSM()}
	p.apply(t, CommandObserveView, view)
	p.apply(t, CommandProposePlan, plan)
	return p
}

func (p *fsmProposer) apply(t *testing.T, commandType CommandType, payload any) {
	t.Helper()
	command, err := NewCommand(commandType, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Propose(command); err != nil {
		t.Fatal(err)
	}
}

func (p *fsmProposer) IsLeader() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.leader
}

func (p *fsmProposer) State() FSMSnapshot { return p.fsm.State() }

func (p *fsmProposer) Propose(command Command) error {
	p.mu.Lock()
	if !p.leader {
		p.mu.Unlock()
		return raft.ErrNotLeader
	}
	p.index++
	index := p.index
	hook := p.afterPropose
	p.mu.Unlock()
	raw, _ := json.Marshal(command)
	if result := p.fsm.Apply(&raft.Log{Index: index, Data: raw}); result != nil {
		return result.(error)
	}
	if hook != nil {
		hook(command)
	}
	return nil
}

type fakeAdminCluster struct {
	mu       sync.Mutex
	state    ClusterStateResponse
	calls    []StepKind
	failNext map[StepKind]int
}

func newFakeAdminCluster(slots []SlotStatus) *fakeAdminCluster {
	return &fakeAdminCluster{state: ClusterStateResponse{Enabled: true, ClusterID: "cluster", SlotCount: uint32(len(slots)), MetadataVersion: 10, Slots: append([]SlotStatus(nil), slots...)}}
}

func (c *fakeAdminCluster) clients(ids ...string) map[string]AdminNodeAPI {
	clients := make(map[string]AdminNodeAPI, len(ids))
	for _, id := range ids {
		clients[id] = &fakeAdminNode{id: id, cluster: c}
	}
	return clients
}

func (c *fakeAdminCluster) snapshot() ClusterStateResponse {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.state
	state.Slots = append([]SlotStatus(nil), c.state.Slots...)
	return state
}

func (c *fakeAdminCluster) callCount(kind StepKind) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, call := range c.calls {
		if call == kind {
			count++
		}
	}
	return count
}

type fakeAdminNode struct {
	id      string
	cluster *fakeAdminCluster
}

type managerExecutorNode struct {
	id        string
	manager   *cluster.Manager
	engine    *engine.Engine
	resp      *server.Server
	cancel    context.CancelFunc
	http      *httptest.Server
	reachable bool
}

func (n *managerExecutorNode) stopData() {
	if n.cancel != nil {
		n.cancel()
		_ = n.resp.Shutdown(context.Background())
		n.cancel = nil
	}
	_ = n.manager.Shutdown(context.Background())
}

func (n *managerExecutorNode) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if !n.reachable {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	state := n.manager.Metadata().Snapshot()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/health":
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok", NodeID: n.id, Mode: "clustered"})
	case r.Method == http.MethodGet && r.URL.Path == "/cluster/state":
		slots := make([]SlotStatus, len(state.Slots))
		for i, slot := range state.Slots {
			slots[i] = SlotStatus{Number: slot.Number, LeaderID: slot.LeaderID, ReplicaID: slot.ReplicaID, Term: slot.Term, ReplicaReady: slot.ReplicaReady}
		}
		_ = json.NewEncoder(w).Encode(ClusterStateResponse{Enabled: true, NodeID: n.id, ClusterID: state.ClusterID, SlotCount: state.SlotCount, MetadataVersion: state.Version, Slots: slots})
	case r.Method == http.MethodPost && r.URL.Path == "/cluster/promote":
		var request struct {
			Slot uint32 `json:"slot"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		updated, failed, err := n.manager.Promote(r.Context(), request.Slot)
		writeManagerAdminResponse(w, n.manager, updated, request.Slot, failed, err)
	case r.Method == http.MethodPost && (r.URL.Path == "/cluster/replica" || r.URL.Path == "/cluster/sync"):
		var request struct {
			Slot   uint32 `json:"slot"`
			NodeID string `json:"nodeId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		var updated cluster.MetadataSnapshot
		var failed []string
		var err error
		if r.URL.Path == "/cluster/replica" {
			updated, failed, err = n.manager.AssignReplica(r.Context(), request.Slot, request.NodeID)
		} else {
			updated, failed, err = n.manager.SyncReplica(r.Context(), request.Slot, request.NodeID)
		}
		writeManagerAdminResponse(w, n.manager, updated, request.Slot, failed, err)
	default:
		http.NotFound(w, r)
	}
}

func writeManagerAdminResponse(w http.ResponseWriter, manager *cluster.Manager, state cluster.MetadataSnapshot, number uint32, failed []string, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	slot, _ := manager.Metadata().Slot(number)
	_ = json.NewEncoder(w).Encode(ClusterAdminResponse{
		MetadataVersion: state.Version,
		Slot:            SlotStatus{Number: slot.Number, LeaderID: slot.LeaderID, ReplicaID: slot.ReplicaID, Term: slot.Term, ReplicaReady: slot.ReplicaReady},
		FailedPeers:     failed,
	})
}

func startManagerExecutorCluster(t *testing.T) ([]*managerExecutorNode, config.ClusterConfig) {
	t.Helper()
	addresses := []string{reserveExecutorAddress(t), reserveExecutorAddress(t), reserveExecutorAddress(t)}
	nodes := make([]*managerExecutorNode, 3)
	for i := range nodes {
		node := &managerExecutorNode{id: fmt.Sprintf("node-%d", i+1), reachable: true}
		node.http = httptest.NewServer(http.HandlerFunc(node.serveHTTP))
		nodes[i] = node
	}
	peers := make([]config.PeerConfig, len(nodes))
	for i, node := range nodes {
		peers[i] = config.PeerConfig{ID: node.id, Address: addresses[i], APIAddress: node.http.URL, FailoverMode: "automatic"}
	}
	cfg := config.ClusterConfig{ID: "executor-test", Enabled: true, ShardingEnabled: true, ReplicationEnabled: true, SlotCount: 3, RoutingMode: "proxy", FailoverMode: "automatic", Peers: peers}
	for i, node := range nodes {
		node.manager = cluster.NewManagerWithNode(cfg, node.id)
		node.engine = engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"})
		node.manager.AttachEngine(node.engine)
		host, portText, _ := net.SplitHostPort(addresses[i])
		var port int
		_, _ = fmt.Sscanf(portText, "%d", &port)
		node.resp = server.New(config.NetworkConfig{BindAddr: host, Port: port, MaxConnections: 32}, node.manager.Coordinator(), metrics.NewNoop())
		ctx, cancel := context.WithCancel(context.Background())
		node.cancel = cancel
		go func(n *managerExecutorNode) { _ = n.resp.Start(ctx) }(node)
	}
	t.Cleanup(func() {
		for _, node := range nodes {
			node.stopData()
			node.http.Close()
		}
	})
	for i, address := range addresses {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			connection, err := net.Dial("tcp", address)
			if err == nil {
				_ = connection.Close()
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if connection, err := net.Dial("tcp", address); err != nil {
			t.Fatalf("node %d did not start: %v", i+1, err)
		} else {
			_ = connection.Close()
		}
	}
	return nodes, cfg
}

func reserveExecutorAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func keyForExactSlot(t *testing.T, metadata *cluster.Metadata, slot uint32) string {
	t.Helper()
	for i := 0; i < 100000; i++ {
		key := fmt.Sprintf("executor:%d", i)
		if metadata.SlotForKey([]byte(key)) == slot {
			return key
		}
	}
	t.Fatal("no key found for slot")
	return ""
}

func viewFromManagerMetadata(state cluster.MetadataSnapshot, failed string) ClusterView {
	view := ClusterView{MetadataVersion: state.Version, Nodes: make(map[string]NodeView, len(state.Peers)), Slots: make([]SlotView, len(state.Slots))}
	for _, peer := range state.Peers {
		if peer.ID == failed {
			view.Nodes[peer.ID] = failedNode(peer.ID)
		} else {
			view.Nodes[peer.ID] = eligibleNode(peer.ID, 0, 0)
		}
	}
	for i, slot := range state.Slots {
		view.Slots[i] = SlotView{Number: slot.Number, LeaderID: slot.LeaderID, ReplicaID: slot.ReplicaID, Term: slot.Term, ReplicaReady: slot.ReplicaReady}
		leader := view.Nodes[slot.LeaderID]
		leader.LeaderSlots++
		view.Nodes[slot.LeaderID] = leader
		replica := view.Nodes[slot.ReplicaID]
		replica.ReplicaSlots++
		view.Nodes[slot.ReplicaID] = replica
	}
	view.Status = summarizeStatus(view)
	return view
}

func (n *fakeAdminNode) Health(context.Context) (HealthResponse, error) {
	return HealthResponse{Status: "ok", NodeID: n.id}, nil
}

func (n *fakeAdminNode) ClusterState(context.Context) (ClusterStateResponse, error) {
	state := n.cluster.snapshot()
	state.NodeID = n.id
	return state, nil
}

func (n *fakeAdminNode) Promote(_ context.Context, slot uint32, _ uint64) (ClusterAdminResponse, error) {
	return n.mutate(StepPromote, slot, "")
}

func (n *fakeAdminNode) AssignReplica(_ context.Context, slot uint32, target string, _ uint64) (ClusterAdminResponse, error) {
	return n.mutate(StepAssignReplica, slot, target)
}

func (n *fakeAdminNode) SyncReplica(_ context.Context, slot uint32, target string, _ uint64) (ClusterAdminResponse, error) {
	return n.mutate(StepSync, slot, target)
}

func (n *fakeAdminNode) mutate(kind StepKind, number uint32, target string) (ClusterAdminResponse, error) {
	c := n.cluster
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, kind)
	if c.failNext[kind] > 0 {
		c.failNext[kind]--
		return ClusterAdminResponse{}, fmt.Errorf("temporary 500")
	}
	slot := &c.state.Slots[number]
	switch kind {
	case StepPromote:
		if n.id != slot.ReplicaID {
			return ClusterAdminResponse{}, fmt.Errorf("%s is not replica", n.id)
		}
		slot.LeaderID, slot.ReplicaID, slot.ReplicaReady = slot.ReplicaID, "", false
		slot.Term++
	case StepAssignReplica:
		if n.id != slot.LeaderID || target == slot.LeaderID {
			return ClusterAdminResponse{}, fmt.Errorf("invalid replica assignment")
		}
		slot.ReplicaID, slot.ReplicaReady = target, false
		slot.Term++
	case StepSync:
		if n.id != slot.LeaderID || target != slot.ReplicaID {
			return ClusterAdminResponse{}, fmt.Errorf("invalid sync")
		}
		slot.ReplicaReady = true
	}
	c.state.MetadataVersion++
	return ClusterAdminResponse{MetadataVersion: c.state.MetadataVersion, Slot: *slot}, nil
}
