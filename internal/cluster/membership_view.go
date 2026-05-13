package cluster

type MemberInfo struct {
	ID      string
	Address string
	State   string
}

func (m *Manager) Membership() []MemberInfo {
	return nil
}
