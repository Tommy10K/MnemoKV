package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
)

func TestObserverBuildsViewAndTracksFailureRecovery(t *testing.T) {
	slots := []SlotStatus{
		{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true},
		{Number: 1, LeaderID: "node-2", ReplicaID: "node-1", Term: 1, ReplicaReady: true},
		{Number: 2, LeaderID: "node-2", ReplicaID: "node-3", Term: 1, ReplicaReady: true},
	}
	fakes, peers, clients := newFakeNodes(t, slots)
	cfg := observerTestConfig(peers)
	observer := NewObserver(cfg, clients, nil)
	now := time.Unix(1000, 0).UTC()
	observer.now = func() time.Time { return now }

	view, err := observer.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Status.State != StatusHealthy || view.Nodes["node-2"].LeaderSlots != 2 || view.Nodes["node-1"].ReplicaSlots != 1 {
		t.Fatalf("unexpected healthy view: %+v", view)
	}
	sets := DeriveTopology(peers, view)
	if len(sets.Configured) != 3 || len(sets.Voters) != 3 || len(sets.Eligible) != 3 || len(sets.Unavailable) != 0 {
		t.Fatalf("unexpected healthy topology: %+v", sets)
	}

	fakes["node-1"].setReachable(false)
	now = now.Add(10 * time.Millisecond)
	view, err = observer.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Status.State != StatusFailureSuspected || !view.Nodes["node-1"].Suspected {
		t.Fatalf("expected suspected failure: %+v", view.Status)
	}
	if ClassifySlots(view)[0] != SlotUnaffected {
		t.Fatal("a suspected node must not change ownership classification")
	}

	now = now.Add(10 * time.Millisecond)
	view, err = observer.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	classes := ClassifySlots(view)
	if view.Status.State != StatusUnavailable || classes[0] != SlotLeaderless || classes[1] != SlotReplicaLost || classes[2] != SlotUnaffected {
		t.Fatalf("unexpected confirmed failure view: status=%+v classes=%+v", view.Status, classes)
	}
	sets = DeriveTopology(peers, view)
	if len(sets.Eligible) != 2 || len(sets.Unavailable) != 1 || sets.Unavailable[0] != "node-1" {
		t.Fatalf("failed node was not excluded from placement: %+v", sets)
	}

	fakes["node-1"].setReachable(true)
	now = now.Add(10 * time.Millisecond)
	view, err = observer.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Status.State != StatusHealthy || view.Nodes["node-1"].ConsecFails != 0 {
		t.Fatalf("node recovery did not reset failure state: %+v", view)
	}
}

func TestObserverSelectsHighestQuorumConsistentMetadata(t *testing.T) {
	slots := []SlotStatus{{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true}}
	fakes, peers, clients := newFakeNodes(t, slots)
	fakes["node-1"].setVersion(5)
	fakes["node-2"].setVersion(5)
	fakes["node-3"].setVersion(8)
	observer := NewObserver(observerTestConfig(peers), clients, nil)
	view, err := observer.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.MetadataVersion != 5 {
		t.Fatalf("metadata version = %d, want quorum-consistent 5", view.MetadataVersion)
	}

	fakes["node-2"].setVersion(8)
	view, err = observer.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.MetadataVersion != 8 {
		t.Fatalf("metadata version = %d, want highest quorum-consistent 8", view.MetadataVersion)
	}
}

func TestObserverConfirmsFailureAfterTimeout(t *testing.T) {
	slots := []SlotStatus{{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true}}
	fakes, peers, clients := newFakeNodes(t, slots)
	cfg := observerTestConfig(peers)
	cfg.Controller.ConsecutiveFailures = 100
	cfg.Controller.FailureTimeoutMs = 50
	observer := NewObserver(cfg, clients, nil)
	now := time.Unix(3000, 0).UTC()
	observer.now = func() time.Time { return now }
	fakes["node-1"].setReachable(false)
	view, err := observer.PollOnce(context.Background())
	if err != nil || view.Status.State != StatusFailureSuspected {
		t.Fatalf("first miss should be suspected: status=%+v err=%v", view.Status, err)
	}
	now = now.Add(51 * time.Millisecond)
	view, err = observer.PollOnce(context.Background())
	if err != nil || view.Status.State != StatusUnavailable {
		t.Fatalf("failure timeout should confirm failure: status=%+v err=%v", view.Status, err)
	}
}

func TestObserverCommitsOnlyMaterialChanges(t *testing.T) {
	slots := []SlotStatus{{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true}}
	_, peers, clients := newFakeNodes(t, slots)
	proposer := &recordingProposer{leader: true}
	observer := NewObserver(observerTestConfig(peers), clients, proposer)
	now := time.Unix(2000, 0).UTC()
	observer.now = func() time.Time { return now }
	if _, err := observer.ObserveAndCommit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(proposer.commands) != 1 {
		t.Fatalf("committed commands = %d, want 1", len(proposer.commands))
	}
	now = now.Add(time.Second)
	if _, err := observer.ObserveAndCommit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(proposer.commands) != 1 {
		t.Fatalf("unchanged view produced another command: %d", len(proposer.commands))
	}
}

func TestTopologyNoSurvivingCopy(t *testing.T) {
	view := ClusterView{
		Nodes: map[string]NodeView{
			"node-1": {ID: "node-1", Reachable: false, Eligible: false},
			"node-2": {ID: "node-2", Reachable: false, Eligible: false},
		},
		Slots: []SlotView{{Number: 9, LeaderID: "node-1", ReplicaID: "node-2", ReplicaReady: true}},
	}
	if ClassifySlots(view)[9] != SlotNoSurvivingCopy {
		t.Fatal("slot with both owners down must not be assigned an empty leader")
	}
	status := summarizeStatus(view)
	if status.State != StatusPotentialDataLoss || status.UnavailableSlots != 1 {
		t.Fatalf("unexpected unavailable status: %+v", status)
	}
}

type fakeNode struct {
	mu        sync.RWMutex
	id        string
	reachable bool
	version   uint64
	slots     []SlotStatus
	server    *httptest.Server
}

func (f *fakeNode) setReachable(reachable bool) {
	f.mu.Lock()
	f.reachable = reachable
	f.mu.Unlock()
}

func (f *fakeNode) setVersion(version uint64) {
	f.mu.Lock()
	f.version = version
	f.mu.Unlock()
}

func (f *fakeNode) serveHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.reachable {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/health":
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok", NodeID: f.id, Mode: "clustered"})
	case "/cluster/state":
		_ = json.NewEncoder(w).Encode(ClusterStateResponse{Enabled: true, NodeID: f.id, ClusterID: "cluster", SlotCount: uint32(len(f.slots)), MetadataVersion: f.version, Slots: f.slots})
	default:
		http.NotFound(w, r)
	}
}

func newFakeNodes(t *testing.T, slots []SlotStatus) (map[string]*fakeNode, []config.PeerConfig, map[string]NodeAPI) {
	t.Helper()
	fakes := make(map[string]*fakeNode, 3)
	peers := make([]config.PeerConfig, 0, 3)
	clients := make(map[string]NodeAPI, 3)
	for i := 1; i <= 3; i++ {
		id := "node-" + string(rune('0'+i))
		fake := &fakeNode{id: id, reachable: true, version: 1, slots: append([]SlotStatus(nil), slots...)}
		fake.server = httptest.NewServer(http.HandlerFunc(fake.serveHTTP))
		t.Cleanup(fake.server.Close)
		client, err := NewNodeClient(fake.server.URL, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		fakes[id] = fake
		peers = append(peers, config.PeerConfig{ID: id, APIAddress: fake.server.URL, FailoverMode: "automatic"})
		clients[id] = client
	}
	return fakes, peers, clients
}

func observerTestConfig(peers []config.PeerConfig) config.ClusterConfig {
	return config.ClusterConfig{
		Peers:      peers,
		Controller: config.ControllerConfig{ObserveIntervalMs: 10, FailureTimeoutMs: 1000, ConsecutiveFailures: 2, RebalanceSkewThreshold: 2},
	}
}

type recordingProposer struct {
	leader   bool
	state    FSMSnapshot
	commands []Command
}

func (p *recordingProposer) IsLeader() bool     { return p.leader }
func (p *recordingProposer) State() FSMSnapshot { return cloneState(p.state) }
func (p *recordingProposer) Propose(command Command) error {
	p.commands = append(p.commands, command)
	if command.Type == CommandObserveView {
		_ = json.Unmarshal(command.Payload, &p.state.LatestView)
	}
	return nil
}
