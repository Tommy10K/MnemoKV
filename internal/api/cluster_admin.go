package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/controlplane"
)

const automaticManagementError = "cluster topology is managed automatically; valid controller authentication is required"

type controllerAuthorization struct {
	index  uint64
	digest [32]byte
}

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
	authorization, ok := s.prepareControllerAuthorization(w, r)
	if !ok {
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
	if replay, release, ok := s.beginControllerOperation(w, authorization); !ok {
		return
	} else if release != nil {
		defer release()
		if replay {
			writeCurrentClusterAdminResult(w, s.cluMgr.Metadata(), req.Slot)
			return
		}
	}
	state, failed, err := s.cluMgr.Promote(r.Context(), req.Slot)
	writeClusterAdminResult(w, s.cluMgr.Metadata(), state, req.Slot, failed, err)
}

func (s *Server) handleClusterReplica(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	authorization, ok := s.prepareControllerAuthorization(w, r)
	if !ok {
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
	if replay, release, ok := s.beginControllerOperation(w, authorization); !ok {
		return
	} else if release != nil {
		defer release()
		if replay {
			writeCurrentClusterAdminResult(w, s.cluMgr.Metadata(), req.Slot)
			return
		}
	}
	state, failed, err := s.cluMgr.AssignReplica(r.Context(), req.Slot, req.NodeID)
	writeClusterAdminResult(w, s.cluMgr.Metadata(), state, req.Slot, failed, err)
}

func (s *Server) handleClusterSync(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	authorization, ok := s.prepareControllerAuthorization(w, r)
	if !ok {
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
	if replay, release, ok := s.beginControllerOperation(w, authorization); !ok {
		return
	} else if release != nil {
		defer release()
		if replay {
			writeCurrentClusterAdminResult(w, s.cluMgr.Metadata(), req.Slot)
			return
		}
	}
	state, failed, err := s.cluMgr.SyncReplica(r.Context(), req.Slot, req.NodeID)
	writeClusterAdminResult(w, s.cluMgr.Metadata(), state, req.Slot, failed, err)
}

func (s *Server) prepareControllerAuthorization(w http.ResponseWriter, r *http.Request) (*controllerAuthorization, bool) {
	if s.cluster.FailoverMode != "automatic" {
		return nil, true
	}
	if s.fenceErr != nil || s.fence == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "automatic control-plane fencing is unavailable"})
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	if err != nil || int64(len(body)) > maxJSONBodyBytes {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return nil, false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	indexText := r.Header.Get(controlplane.ControlIndexHeader)
	signature := r.Header.Get(controlplane.ControlSignatureHeader)
	if !controlplane.Verify([]byte(s.controlPlane.RequestSigningSecret), r.Method, r.URL.Path, body, indexText, signature) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": automaticManagementError})
		return nil, false
	}
	index, err := strconv.ParseUint(indexText, 10, 64)
	if err != nil || index == 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": automaticManagementError})
		return nil, false
	}
	return &controllerAuthorization{index: index, digest: controlplane.OperationDigest(r.Method, r.URL.Path, body, indexText)}, true
}

func (s *Server) beginControllerOperation(w http.ResponseWriter, authorization *controllerAuthorization) (bool, func(), bool) {
	if authorization == nil {
		return false, nil, true
	}
	replay, release, err := s.fence.Begin(authorization.index, authorization.digest)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, controlplane.ErrStaleControlIndex) || errors.Is(err, controlplane.ErrControlIndexConflict) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return false, nil, false
	}
	return replay, release, true
}

func writeCurrentClusterAdminResult(w http.ResponseWriter, metadata *cluster.Metadata, slot uint32) {
	if metadata == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster mode is disabled"})
		return
	}
	state := metadata.Snapshot()
	writeClusterAdminResult(w, metadata, state, slot, nil, nil)
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
