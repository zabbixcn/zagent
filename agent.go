// The zagent package allows you to query zabbix agents running over a network.
package zagent

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"time"
)

var (
	// http://localhost:6060/pkg/encoding/binary/#Uvarint
	DataLengthBufferTooSmall = errors.New("DataLength buffer too small")
	DataLengthOverflow       = errors.New("DataLength is too large")

	// This is the default timeout when contacting a Zabbix Agent.
	DefaultTimeout = time.Duration(30 * time.Second)
)

const (
	NotSupported = "ZBX_NOTSUPPORTED"
)

// Creates a new Agent with a default port of 10050
func NewAgent(host string) *Agent {
	return &Agent{Host: host, Port: 10050}
}

// Agent represents a remote zabbix agent
type Agent struct {
	Host string
	Port int
}

// Returns a string with the host and port concatenated to host:port
func (a *Agent) hostPort() string {
	portS := fmt.Sprintf("%v", a.Port)
	return net.JoinHostPort(a.Host, portS)
}

// Run the check (key) against the Zabbix agent with the default timeout
func (a *Agent) Get(key string) (*Response, error) {
	return a.GetTimeout(key, DefaultTimeout)
}

// Run the check (key) against the Zabbix agent with the specified timeout
func (a *Agent) GetTimeout(key string, timeout time.Duration) (*Response, error) {
	res := newResponse()

	conn, err := net.DialTimeout("tcp", a.hostPort(), timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_, err = fmt.Fprintf(conn, key)
	if err != nil {
		return nil, err
	}

	dataLength := make([]byte, 8)

	reader := bufio.NewReader(conn)
	reader.Read(res.Header)
	reader.Read(dataLength)
	res.Data, _ = ioutil.ReadAll(reader)

	// Convert dataLength from binary to uint
	var bytesRead int
	res.DataLength, bytesRead = binary.Uvarint(dataLength)
	if bytesRead <= 0 {
		if bytesRead == 0 {
			return nil, DataLengthBufferTooSmall
		}
		return nil, DataLengthOverflow
	}

	if res.Supported() == false {
		return res, fmt.Errorf("%s is not supported", key)
	}

	return res, nil
}

// Call agent.ping on the zabbix agent. Returns true if it
// gets the correct response ("1") and doesn't receive any
// errors in the process.
func (a *Agent) Ping() (bool, error) {
	return a.PingTimeout(DefaultTimeout)
}

// Same as Agent.Ping() but allows a timeout to be specified.
func (a *Agent) PingTimeout(timeout time.Duration) (bool, error) {
	res, err := a.GetTimeout("agent.ping", timeout)
	if err != nil {
		return false, err
	}

	if res.Supported() && res.DataAsString() == "1" {
		return true, nil
	}

	return false, nil
}

// Calls agent.hostname on the zabbix agent and returns the hostname
// and/or any errors associated with the action.
func (a *Agent) Hostname() (string, error) {
	return a.HostnameTimeout(DefaultTimeout)
}

// Same as Agent.Hostname() but called with the timeout specified.
func (a *Agent) HostnameTimeout(timeout time.Duration) (string, error) {
	res, err := a.GetTimeout("agent.hostname", timeout)
	if err != nil {
		return "", err
	}

	return res.DataAsString(), nil
}

// Calls agent.version on the zabbix agent and returns the version
// and/or any errors associated with the action.
func (a *Agent) Version() (string, error) {
	return a.VersionTimeout(DefaultTimeout)
}

// Same as Agent.Version() but called with the timeout specified.
func (a *Agent) VersionTimeout(timeout time.Duration) (string, error) {
	res, err := a.GetTimeout("agent.version", timeout)
	if err != nil {
		return "", err
	}

	return res.DataAsString(), nil
}

// Response is the response from the zabbix agent.
// Response.Data is generally what most people care
// about. See the wire format here:
// https://www.zabbix.com/documentation/2.2/manual/appendix/items/activepassive
type Response struct {
	Header     []byte // This should always be: ZBXD\x01
	DataLength uint64 // The size of the response
	Data       []byte // The results of the query
	key        string
}

// Returns true if the key is supported, false if it wasn't.
// Most of the time you shouldn't need to call this as Agent.Get()
// will return an error if the key is unsupported.
func (r *Response) Supported() bool {
	return r.DataAsString() != NotSupported
}

// Returns the key that was used in the query against the Zabbix agent.
func (r *Response) Key() string {
	return r.key
}

// Convenience wrapper to return Data as a string.
func (r *Response) DataAsString() string {
	return string(r.Data)
}

// Convenience wrapper to return Data as an int.
func (r *Response) DataAsInt() (int, error) {
	return strconv.Atoi(r.DataAsString())
}

// Convenience wrapper to return Data as an int64.
func (r *Response) DataAsInt64() (int64, error) {
	return strconv.ParseInt(r.DataAsString(), 10, 64)
}

// Convenience wrapper to return Data as uint64.
func (r *Response) DataAsUint64() (uint64, error) {
	return strconv.ParseUint(r.DataAsString(), 10, 64)
}

// Convenience wrapper to return Data as a float64.
func (r *Response) DataAsFloat64() (float64, error) {
	return strconv.ParseFloat(r.DataAsString(), 64)
}

// Create a new Response type
func newResponse() *Response {
	return &Response{
		// Header is always 5 bytes
		Header: make([]byte, 5),
	}
}
