package eviction

func PolicyByName(name string) Policy {
	switch name {
	case "fifo":
		return FIFO{}
	case "random":
		return Random{}
	case "lru":
		return LRU{}
	case "lfu":
		return LFU{}
	default:
		return Noop{}
	}
}
