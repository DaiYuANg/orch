package pkg

import (
	"log"
	"net"
)

func GetFreePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func(l net.Listener) {
		err := l.Close()
		if err != nil {
			log.Println(err)
		}
	}(l)

	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
