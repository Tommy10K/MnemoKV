// Package controller owns MnemoKV's optional automatic-recovery control plane.
//
// The package is deliberately isolated from the RESP/HTTP command path. It may
// observe node APIs and drive existing cluster administration operations, but
// cluster.Metadata remains authoritative and synchronous data replication is
// unchanged. Automatic recovery is disabled unless failoverMode is automatic.
package controller
