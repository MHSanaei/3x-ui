// +build linux

package sys

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

func getLinesNum(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	sum := 0
	buf := make([]byte, 8192)
	for {
		n, err := file.Read(buf)

		var buffPosition int
		for {
			i := bytes.IndexByte(buf[buffPosition:], '\n')
			if i < 0 || n == buffPosition {
				break
			}
			buffPosition += i + 1
			sum++
		}

		if err == io.EOF {
			return sum, nil
		} else if err != nil {
			return sum, err
		}
	}
}

func GetTCPCount() (int, error) {
	root := HostProc()

	tcp4, err := getLinesNum(fmt.Sprintf("%v/net/tcp", root))
	if err != nil {
		return tcp4, err
	}
	tcp6, err := getLinesNum(fmt.Sprintf("%v/net/tcp6", root))
	if err != nil {
		return tcp4 + tcp6, nil
	}

	return tcp4 + tcp6, nil
}

func GetUDPCount() (int, error) {
	root := HostProc()

	udp4, err := getLinesNum(fmt.Sprintf("%v/net/udp", root))
	if err != nil {
		return udp4, err
	}
	udp6, err := getLinesNum(fmt.Sprintf("%v/net/udp6", root))
	if err != nil {
		return udp4 + udp6, nil
	}

	return udp4 + udp6, nil
}
