package fnv

// borrow from https://github.com/prometheus/client_golang

// Inline and byte-free variant of hash/fnv's fnv64a.

const (
	offset64 = 14695981039346656037
	prime64  = 1099511628211
)

func NewHash() uint64 {
	return offset64
}

func AddString(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h = Add(h, uint64(s[i]))
	}
	return h
}

func Add(h, i uint64) uint64 {
	h ^= i
	h *= prime64
	return h
}
