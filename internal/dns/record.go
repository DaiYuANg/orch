package dns

import "time"

type Record struct {
	Domain     string    `json:"domain"`     // FQDN，以 “.” 结尾，如 “foo.example.com.”
	Type       string    `json:"type"`       // DNS 记录类型，A / AAAA / CNAME 等
	Value      string    `json:"value"`      // IPv4 / IPv6 / alias
	TTLSeconds int       `json:"ttlSeconds"` // DNS TTL（秒）
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Metadata   string    `json:"metadata,omitempty"` // 可附加 owner、备注等
}
