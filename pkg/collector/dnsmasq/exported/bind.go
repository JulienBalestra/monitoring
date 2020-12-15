package exported

const (
	dnsQueryBindSuffix = ".bind"

	HitsQueryBind       = "hits" + dnsQueryBindSuffix
	MissesQueryBind     = "misses" + dnsQueryBindSuffix
	EvictionsQueryBind  = "evictions" + dnsQueryBindSuffix
	InsertionsQueryBind = "insertions" + dnsQueryBindSuffix
	CachesizeQueryBind  = "cachesize" + dnsQueryBindSuffix
)
