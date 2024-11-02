package http

import (
	"net"

	"github.com/crimist/trakx/utils"
)

func redir(c net.Conn, url string) {
	c.Write(utils.StringToBytesUnsafe("HTTP/1.1 303\r\nLocation: " + url + "\r\n\r\n"))
}

func writeData(c net.Conn, data string) {
	c.Write(utils.StringToBytesUnsafe("HTTP/1.1 200\r\n\r\n" + data))
}

func writeDataBytes(c net.Conn, data []byte) {
	c.Write(append([]byte("HTTP/1.1 200\r\n\r\n"), data...))
}

func writeStatus(c net.Conn, status string) {
	c.Write(utils.StringToBytesUnsafe("HTTP/1.1 " + status + "\r\n\r\n"))
}
