package workload

type Profile struct {
	Name        string
	Description string
	Operations  []Operation
}

type Operation struct {
	Weight int
	Build  func(rand RandSource) []string
}

func StringProfile() Profile {
	return Profile{
		Name:        "strings",
		Description: "Set/Get heavy workload",
		Operations: []Operation{
			{Weight: 60, Build: func(r RandSource) []string {
				return []string{"SET", r.Key("k"), r.Value(64)}
			}},
			{Weight: 35, Build: func(r RandSource) []string {
				return []string{"GET", r.Key("k")}
			}},
			{Weight: 5, Build: func(r RandSource) []string {
				return []string{"INCR", r.CounterKey("c")}
			}},
		},
	}
}

func ListProfile() Profile {
	return Profile{
		Name:        "lists",
		Description: "Queue-style list workload",
		Operations: []Operation{
			{Weight: 50, Build: func(r RandSource) []string {
				return []string{"RPUSH", r.Key("q"), r.Value(32)}
			}},
			{Weight: 30, Build: func(r RandSource) []string {
				return []string{"LPOP", r.Key("q")}
			}},
			{Weight: 20, Build: func(r RandSource) []string {
				return []string{"LLEN", r.Key("q")}
			}},
		},
	}
}

func ZSetProfile() Profile {
	return Profile{
		Name:        "zset",
		Description: "Leaderboard-style sorted set workload",
		Operations: []Operation{
			{Weight: 60, Build: func(r RandSource) []string {
				return []string{"ZADD", "leaderboard", r.Score(), r.Key("user")}
			}},
			{Weight: 30, Build: func(r RandSource) []string {
				return []string{"ZRANGE", "leaderboard", "0", "9"}
			}},
			{Weight: 10, Build: func(r RandSource) []string {
				return []string{"ZCARD", "leaderboard"}
			}},
		},
	}
}

func MixedProfile() Profile {
	return Profile{
		Name:        "mixed",
		Description: "Mixed read/write workload across data types",
		Operations:  append(append(StringProfile().Operations, ListProfile().Operations...), ZSetProfile().Operations...),
	}
}

func ProfileByName(name string) (Profile, bool) {
	switch name {
	case "strings":
		return StringProfile(), true
	case "lists":
		return ListProfile(), true
	case "zset":
		return ZSetProfile(), true
	case "mixed":
		return MixedProfile(), true
	}
	return Profile{}, false
}
