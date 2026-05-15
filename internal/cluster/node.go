package cluster

type Node struct {
	ID      string
	Address string
}

type Route struct {
	OwnerNodeID string
	Slot        uint16
	IsLocal     bool
}
