package api

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/persistence"
	"github.com/mnemokv/mnemokv/internal/resp"
	"github.com/mnemokv/mnemokv/internal/snapshot"
)

func TestReturningNodePrepareClearsDataAndSnapshotsBeforeAdmission(t *testing.T) {
	server := newClusterAdminTestServer(t, "automatic", t.TempDir())
	dir := t.TempDir()
	snapshots := persistence.New(config.PersistenceConfig{
		Enabled: true, DataDir: dir, SnapshotIntervalSec: 60, MaxSnapshots: 2, Format: snapshot.FormatJSON,
	}, "node-1", server.engine, server.cluMgr.SnapshotMetadata)
	server.snapshots = snapshots
	if frame := server.engine.ApplyReplicated(&resp.Command{Name: "SET", Args: [][]byte{[]byte("stale"), []byte("value")}}); frame != resp.OK {
		t.Fatal(frame)
	}
	result, err := snapshots.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	server.cluMgr.RequireAdmission()
	state := server.cluMgr.Metadata().Snapshot()
	body, _ := json.Marshal(returningNodeRequest{ClusterID: state.ClusterID, MetadataVersion: state.Version})
	response := callClusterAdmin(server.handleReturningNodePrepare, "/cluster/returning/prepare", body, 1, "secret", "")
	if response.Code != http.StatusOK {
		t.Fatalf("prepare = %d %s", response.Code, response.Body.String())
	}
	wantGate := resp.Error{Prefix: "CLUSTERDOWN", Message: "node is recovering and not admitted"}
	if frame := server.cluMgr.Coordinator().Execute(&resp.Command{Name: "GET", Args: [][]byte{[]byte("stale")}}); frame != wantGate {
		t.Fatalf("node served before admission: %#v", frame)
	}
	if _, err := os.Stat(result.Path); !os.IsNotExist(err) {
		t.Fatalf("obsolete snapshot still exists: %v", err)
	}
	response = callClusterAdmin(server.handleReturningNodeAdmit, "/cluster/returning/admit", body, 2, "secret", "")
	if response.Code != http.StatusOK || server.cluMgr.DataState() != "active" {
		t.Fatalf("admit = %d %s state=%s", response.Code, response.Body.String(), server.cluMgr.DataState())
	}
	if frame := server.engine.Execute(&resp.Command{Name: "GET", Args: [][]byte{[]byte("stale")}}); func() bool { bulk, ok := frame.(resp.BulkString); return ok && bulk.Null }() == false {
		t.Fatalf("stale key resurrected: %#v", frame)
	}
}
