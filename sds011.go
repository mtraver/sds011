// Package sds011 reads measurements from the SDS011 particulate matter sensor.
// Datasheet: https://www-sd-nf.oss-cn-beijing.aliyuncs.com/%E5%AE%98%E7%BD%91%E4%B8%8B%E8%BD%BD/SDS011%20laser%20PM2.5%20sensor%20specification-V1.4.pdf
package sds011

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
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

func (m Measurement) String() string {
	return fmt.Sprintf("PM2.5 = %v μg/m³  PM10 = %v μg/m³", m.PM25, m.PM10)
}

type Dev struct {
	port       *serial.Port
	id         uint16
	stopListen bool

	// readTimeout is the timeout used in readAndValidate.
	readTimeout time.Duration

	mu       sync.Mutex
	doneChan chan struct{}
}

type Mode byte

type command byte

type commandType byte

var (
	ModeActive Mode = 0x00
	ModeQuery  Mode = 0x01

	modeCommand            command = 0x02
	queryCommand           command = 0x04
	deviceIDCommand        command = 0x05
	sleepWorkCommand       command = 0x06
	workingPeriodCommand   command = 0x08
	firmwareVersionCommand command = 0x07

	cmdTypeQuery   commandType = 0xc0
	cmdTypeGeneral commandType = 0xc5

	head byte = 0xaa
	tail byte = 0xab

	errTimeout = fmt.Errorf("sds011: read timeout")

	defaultTimeout = 2 * time.Second
)

type Handler func(Measurement)

func New(name string) (Dev, error) {
	port, err := serial.Open(name, serial.WithBaudrate(9600), serial.WithDataBits(8),
		serial.WithParity(serial.NoParity), serial.WithStopBits(serial.OneStopBit))
	if err != nil {
		return Dev{}, err
	}

	// Without a timeout Read returns immediately.
	port.SetReadTimeout(250)

	return Dev{
		port:        port,
		id:          0xffff,
		readTimeout: defaultTimeout,
	}, nil
}

func (d *Dev) sense() (Measurement, error) {
	buf, err := d.readAndValidate(cmdTypeQuery, queryCommand)
	if err != nil {
		return Measurement{}, err
	}
	return unmarshal(buf)
}

func (d *Dev) Sense() (Measurement, error) {
	cmd := []byte{byte(queryCommand)}
	if err := d.write(cmd); err != nil {
		return Measurement{}, err
	}

	return d.sense()
}

func (d *Dev) Listen(h Handler) error {
	d.mu.Lock()
	if d.doneChan != nil {
		d.mu.Unlock()
		return fmt.Errorf("sds011: already listening")
	}

	d.doneChan = make(chan struct{})
	d.mu.Unlock()

	for {
		select {
		case <-d.doneChan:
			d.mu.Lock()
			defer d.mu.Unlock()

			// Reset the channel so Listen can be called again.
			d.doneChan = nil
			return nil
		default:
		}

		m, err := d.sense()
		if err == errTimeout {
			continue
		} else if err != nil {
			return err
		}
		go h(m)
	}
}

func (d *Dev) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// If the channel is nil then we're not currently listening.
	if d.doneChan == nil {
		return
	}

	select {
	case <-d.doneChan:
		// Already closed. Don't close again.
	default:
		// Safe to close. We're the only closer, guarded by d.mu.
		close(d.doneChan)
	}
}

func (d *Dev) SetMode(m Mode) error {
	cmd := []byte{byte(modeCommand), 0x01, byte(m)}
	if err := d.write(cmd); err != nil {
		return err
	}

	_, err := d.readAndValidate(cmdTypeGeneral, modeCommand)
	return err
}

func (d *Dev) SetDeviceID(id uint16) error {
	cmd := make([]byte, 11)
	cmd[0] = byte(deviceIDCommand)
	cmd = append(cmd, toBytes(id)...)
	if err := d.write(cmd); err != nil {
		return err
	}

	_, err := d.readAndValidate(cmdTypeGeneral, deviceIDCommand)
	return err
}

func (d *Dev) sleepWake(sw byte) error {
	cmd := []byte{byte(sleepWorkCommand), 0x01, sw}
	if err := d.write(cmd); err != nil {
		return err
	}

	_, err := d.readAndValidate(cmdTypeGeneral, sleepWorkCommand)
	return err
}

func (d *Dev) Sleep() error {
	return d.sleepWake(0x00)
}

func (d *Dev) Wake() error {
	return d.sleepWake(0x01)
}

func (d *Dev) SetPeriod(minutes int) error {
	if minutes < 0 || minutes > 30 {
		return fmt.Errorf("sds011: working period must be in [0, 30]")
	}

	cmd := []byte{byte(workingPeriodCommand), 0x01, byte(minutes)}
	if err := d.write(cmd); err != nil {
		return err
	}

	_, err := d.readAndValidate(cmdTypeGeneral, workingPeriodCommand)
	return err
}

func (d *Dev) GetFirmwareVersion() ([]byte, error) {
	cmd := []byte{byte(firmwareVersionCommand)}
	if err := d.write(cmd); err != nil {
		return []byte{}, err
	}

	b, err := d.readAndValidate(cmdTypeGeneral, firmwareVersionCommand)
	if err != nil {
		return nil, err
	}
	return b[3:6], nil
}

func (d *Dev) write(b []byte) error {
	data := make([]byte, 13)
	copy(data, b)

	var buf bytes.Buffer
	buf.Write([]byte{head, 0xb4})
	buf.Write(data)
	buf.Write(toBytes(d.id))
	buf.WriteByte(checksum(append(data, toBytes(d.id)...)))
	buf.WriteByte(tail)

	_, err := d.port.Write(buf.Bytes())
	return err
}

func (d *Dev) read() ([]byte, error) {
	packet := make([]byte, packetLength)
	n, err := d.port.Read(packet)
	if err != nil {
		return nil, err
	}
	if n != packetLength {
		return nil, fmt.Errorf("sds011: read: bad packet length, got %v, expected %v", n, packetLength)
	}

	// Do just enough validation to determine that the structure of the packet is valid.
	if packet[0] != head {
		return nil, fmt.Errorf("sds011: bad header")
	}
	if !contains([]byte{byte(cmdTypeQuery), byte(cmdTypeGeneral)}, packet[1]) {
		return nil, fmt.Errorf("sds011: bad command type")
	}
	if packet[packetLength-1] != tail {
		return nil, fmt.Errorf("sds011: bad tail")
	}

	return packet, nil
}

func (d *Dev) readAndValidate(typ commandType, cmd command) ([]byte, error) {
	start := time.Now()

	b, err := d.read()
	for err != nil || validate(b, typ, cmd) != nil {
		if time.Now().Sub(start) > d.readTimeout {
			return b, errTimeout
		}

		b, err = d.read()
	}
	return b, err
}

func unmarshal(b []byte) (Measurement, error) {
	if len(b) != packetLength {
		return Measurement{}, fmt.Errorf("sds011: bad packet length, got %v, expected %v", len(b), packetLength)
	}

	return Measurement{
		PM25: float32(binary.LittleEndian.Uint16(b[2:4])) / 10,
		PM10: float32(binary.LittleEndian.Uint16(b[4:6])) / 10,
	}, nil
}

func validate(b []byte, typ commandType, cmd command) error {
	if len(b) != packetLength {
		return fmt.Errorf("sds011: validate: bad packet length, got %v, expected %v", len(b), packetLength)
	}

	if b[0] != head {
		return fmt.Errorf("sds011: bad header")
	}
	if b[1] != byte(typ) {
		return fmt.Errorf("sds011: incorrect command type, got 0x%x, want 0x%x", b[1], byte(typ))
	}

	// Query responses don't include the command byte because all the space is taken up by the measurement data.
	if typ != cmdTypeQuery && b[2] != byte(cmd) {
		return fmt.Errorf("sds011: incorrect command ID, got 0x%x, want 0x%x", b[2], byte(cmd))
	}

	if b[9] != tail {
		return fmt.Errorf("sds011: bad tail")
	}

	if b[8] != checksum(b[2:8]) {
		return fmt.Errorf("sds011: bad checksum")
	}

	return nil
}

func checksum(b []byte) byte {
	var sum int
	for _, v := range b {
		sum += int(v)
	}

	return byte(sum & 0xff)
}

func toBytes(u uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, u)
	return b
}

func contains(s []byte, b byte) bool {
	for _, e := range s {
		if b == e {
			return true
		}
	}
	return false
}

func fmtBytes(s []byte) string {
	if s == nil {
		return "<nil>"
	}
	if len(s) == 0 {
		return "[]"
	}
	if len(s) == 1 {
		return fmt.Sprintf("[0x%x]", s[0])
	}

	strs := make([]string, len(s))
	for i, b := range s {
		if i == 0 {
			strs[i] = fmt.Sprintf("[0x%x", b)
		} else if i == len(s)-1 {
			strs[i] = fmt.Sprintf("0x%x]", b)
		} else {
			strs[i] = fmt.Sprintf("0x%x", b)
		}
	}
	return strings.Join(strs, ", ")
}
