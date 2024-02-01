package powerdns_recursor

import (
	"flashcat.cloud/categraf/types"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"time"
)

// V1 (before 4.5.0) Protocol:
// Unix datagram socket
// Synchronous request / response, individual datagrams
// Structure:
// data: byte[]
// The `data` field contains a list of commands to execute with
// the \n character after every command.
func (ins *Instance) gatherFromV1Server(address string, slist *types.SampleList) error {
	randomNumber := rand.Int63()
	recvSocket := filepath.Join(ins.SocketDir, fmt.Sprintf("pdns_recursor_telegraf%d", randomNumber))

	laddr, err := net.ResolveUnixAddr("unixgram", recvSocket)
	if err != nil {
		return err
	}

	defer os.Remove(recvSocket)

	raddr, err := net.ResolveUnixAddr("unixgram", address)
	if err != nil {
		return err
	}

	conn, err := net.DialUnix("unixgram", laddr, raddr)
	if err != nil {
		return err
	}

	defer conn.Close()

	if err := os.Chmod(recvSocket, os.FileMode(ins.mode)); err != nil {
		return err
	}

	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return err
	}

	// Then send the get-all command.
	command := "get-all\n"

	_, err = conn.Write([]byte(command))
	if err != nil {
		return err
	}

	// Read the response data.
	buf := make([]byte, 16_384)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no data received")
	}

	metrics := string(buf)

	// Process data
	fields := parseResponse(metrics)

	// Add server socket as a tag
	tags := map[string]string{"server": address}

	//acc.AddFields("powerdns_recursor", fields, tags)

	slist.PushSamples("powerdns_recursor", fields, tags)

	return nil
}
