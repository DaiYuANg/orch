//go:build darwin

package tun

func pickInterfaceName(user string) string {
	if user != "" {
		return user
	}
	// Lets the kernel assign the next utun unit (wireguard/tun special-case).
	return "utun"
}
