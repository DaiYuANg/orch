package hostdns

import "fmt"

func unsupportedError(goos string) error {
	return fmt.Errorf("host DNS installer is not supported on %s", goos)
}
