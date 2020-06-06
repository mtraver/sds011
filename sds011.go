// Package sds011 reads measurements from the SDS011 particulate matter sensor.
// Datasheet: https://www-sd-nf.oss-cn-beijing.aliyuncs.com/%E5%AE%98%E7%BD%91%E4%B8%8B%E8%BD%BD/SDS011%20laser%20PM2.5%20sensor%20specification-V1.4.pdf
package sds011

import (
	"fmt"
	"time"

	serial "github.com/albenik/go-serial/v2"
)

const (
	packetLength = 10
)

type Measurement struct {
	PM25 float32
	PM10 float32
}

type Dev struct {
	port *serial.Port
}

func New(name string) (Dev, error) {
	port, err := serial.Open(name, serial.WithBaudrate(9600), serial.WithDataBits(8),
		serial.WithParity(serial.NoParity), serial.WithStopBits(serial.OneStopBit))
	if err != nil {
		return Dev{}, err
	}

	return Dev{
		port: port,
	}, nil
}

func unmarshal(buf []byte) (Measurement, error) {
	m := Measurement{}

	if len(buf) != packetLength {
		return m, fmt.Errorf("sds011: bad packet length, got %v, expected %v", len(buf), packetLength)
	}

	// Verify constant parts of packet.
	if buf[0] != byte('\xaa') {
		return m, fmt.Errorf("sds011: bad header")
	}
	if buf[1] != byte('\xc0') {
		return m, fmt.Errorf("sds011: bad command number")
	}
	if buf[9] != byte('\xab') {
		return m, fmt.Errorf("sds011: bad tail")
	}

	// Verify checksum.
	cs := buf[2]
	for i := 3; i < 8; i++ {
		cs += buf[i]
	}
	if buf[8] != cs {
		return m, fmt.Errorf("sds011: bad checksum")
	}

	m.PM25 = ((float32(buf[3]) * 256) + float32(buf[2])) / 10
	m.PM10 = ((float32(buf[5]) * 256) + float32(buf[4])) / 10

	return m, nil
}

func (d Dev) Sense() (Measurement, error) {
	// Read until we get a packet. The sensor emits measurements at 1Hz so if we
	// read in between them we'll get 0 bytes back. Instead of returning an error
	// in that case wait until the next one arrives.
	buf := make([]byte, packetLength)
	n := 0
	var err error
	for n != packetLength {
		n, err = d.port.Read(buf)
		if err != nil {
			return Measurement{}, err
		}

		time.Sleep(100 * time.Millisecond)
	}

	return unmarshal(buf)
}
