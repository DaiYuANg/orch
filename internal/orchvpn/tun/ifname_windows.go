//go:build windows

package tun

func pickInterfaceName(user string) string {
	if user != "" {
		return user
	}
	return "orch-vpn"
}
