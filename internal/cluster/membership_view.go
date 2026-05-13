package cluster

type MemberInfo struct {
	ID      string
	Address string
	State   string
}

func (m *Manager) Membership() []MemberInfo {
	if m.membership == nil {
		return nil
	}
	return m.membership.Snapshot()
}
