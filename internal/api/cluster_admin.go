package api

import (
	"net/http"

	"github.com/mnemokv/mnemokv/internal/cluster"
)

type clusterSlotRequest struct {
	Slot uint32 `json:"slot"`
}

type clusterReplicaRequest struct {
	Slot   uint32 `json:"slot"`
	NodeID string `json:"nodeId"`
}

func (s *Server) handleClusterPromote(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req clusterSlotRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	if !s.cluster.Enabled || s.cluMgr == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster mode is disabled"})
		return
	}
	state, failed, err := s.cluMgr.Promote(r.Context(), req.Slot)
	writeClusterAdminResult(w, s.cluMgr.Metadata(), state, req.Slot, failed, err)
}

func (s *Server) handleClusterReplica(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req clusterReplicaRequest
	if err := decodeJSONBody(w, r, &req); err != nil || req.NodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	if !s.cluster.Enabled || s.cluMgr == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster mode is disabled"})
		return
	}
	state, failed, err := s.cluMgr.AssignReplica(r.Context(), req.Slot, req.NodeID)
	writeClusterAdminResult(w, s.cluMgr.Metadata(), state, req.Slot, failed, err)
}

func (s *Server) handleClusterSync(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req clusterReplicaRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	if !s.cluster.Enabled || s.cluMgr == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster mode is disabled"})
		return
	}
	state, failed, err := s.cluMgr.SyncReplica(r.Context(), req.Slot, req.NodeID)
	writeClusterAdminResult(w, s.cluMgr.Metadata(), state, req.Slot, failed, err)
}

func writeClusterAdminResult(w http.ResponseWriter, metadata *cluster.Metadata, state cluster.MetadataSnapshot, slot uint32, failed []string, err error) {
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	slotState, ok := metadata.Slot(slot)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "updated slot is missing"})
		return
	}
	writeJSON(w, http.StatusOK, ClusterAdminResponse{MetadataVersion: state.Version, Slot: slotStatus(metadata, slotState), FailedPeers: failed})
}

func slotStatus(metadata *cluster.Metadata, slot cluster.SlotState) SlotStatus {
	return SlotStatus{
		Number: slot.Number, LeaderID: slot.LeaderID, ReplicaID: slot.ReplicaID,
		LocalRole: metadata.LocalRole(slot), Term: slot.Term, LastSequence: slot.LastSequence,
		LastAppliedSequence: slot.LastAppliedSequence, ReplicaReady: slot.ReplicaReady,
	}
}
