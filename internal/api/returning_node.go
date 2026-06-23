package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/mnemokv/mnemokv/internal/persistence"
)

type returningNodeRequest struct {
	ClusterID       string `json:"clusterId"`
	MetadataVersion uint64 `json:"metadataVersion"`
}

type snapshotInvalidator interface {
	Invalidate() (int, error)
}

func (s *Server) handleReturningNodePrepare(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	authorization, ok := s.prepareControllerAuthorization(w, r)
	if !ok {
		return
	}
	var req returningNodeRequest
	if err := decodeJSONBody(w, r, &req); err != nil || req.ClusterID == "" || req.MetadataVersion == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	if s.cluster.FailoverMode != "automatic" || s.cluMgr == nil || !s.cluster.Enabled {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "returning-node admission requires automatic cluster mode"})
		return
	}
	if replay, release, ok := s.beginControllerOperation(w, authorization); !ok {
		return
	} else if release != nil {
		defer release()
		if replay {
			s.writeReturningNodeState(w, 0)
			return
		}
	}
	s.cluMgr.RequireAdmission()
	if _, err := s.engine.RestoreSnapshotEntries(nil, time.Now()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	removed := 0
	if invalidator, ok := s.snapshots.(snapshotInvalidator); ok {
		var err error
		removed, err = invalidator.Invalidate()
		if err != nil && !errors.Is(err, persistence.ErrDisabled) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	s.cluMgr.RefreshMetadata(r.Context())
	state := s.cluMgr.Metadata().Snapshot()
	if state.ClusterID != req.ClusterID || state.Version < req.MetadataVersion {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "latest committed cluster metadata is not installed"})
		return
	}
	s.writeReturningNodeState(w, removed)
}

func (s *Server) handleReturningNodeAdmit(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	authorization, ok := s.prepareControllerAuthorization(w, r)
	if !ok {
		return
	}
	var req returningNodeRequest
	if err := decodeJSONBody(w, r, &req); err != nil || req.ClusterID == "" || req.MetadataVersion == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	if s.cluster.FailoverMode != "automatic" || s.cluMgr == nil || !s.cluster.Enabled {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "returning-node admission requires automatic cluster mode"})
		return
	}
	if replay, release, ok := s.beginControllerOperation(w, authorization); !ok {
		return
	} else if release != nil {
		defer release()
		if replay {
			s.writeReturningNodeState(w, 0)
			return
		}
	}
	state := s.cluMgr.Metadata().Snapshot()
	entries, err := s.engine.SnapshotEntries()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if state.ClusterID != req.ClusterID || state.Version < req.MetadataVersion || len(entries) != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "returning node validation failed"})
		return
	}
	s.cluMgr.AdmitData()
	s.writeReturningNodeState(w, 0)
}

func (s *Server) writeReturningNodeState(w http.ResponseWriter, removed int) {
	state := s.cluMgr.Metadata().Snapshot()
	entries, err := s.engine.SnapshotEntries()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ReturningNodeResponse{
		ClusterID: state.ClusterID, MetadataVersion: state.Version, EntryCount: len(entries),
		RemovedSnapshots: removed, DataState: s.cluMgr.DataState(),
	})
}
