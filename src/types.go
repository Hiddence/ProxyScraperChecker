package src

type ProxyType int

const (
	ProxyTypeHTTP ProxyType = iota
	ProxyTypeSOCKS5
)

func (t ProxyType) String() string {
	switch t {
	case ProxyTypeHTTP:
		return "HTTP"
	case ProxyTypeSOCKS5:
		return "SOCKS5"
	default:
		return "Unknown"
	}
} 