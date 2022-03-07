package discovery

import (
	"crypto/ecdsa"
	"github.com/bloxapp/ssv/network/commons"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
)

// DiscV5Options for creating a new discv5 listener
type DiscV5Options struct {
	// StoragePath is the path used to store the DB (DHT)
	// if an empty path was given, the DB will be created in memory
	StoragePath string
	// IP of the node
	IP string
	// BindIP is the IP to bind to the UDP listener
	BindIP string
	// Port is the UDP port used by discv5
	Port int
	// TCPPort is the TCP port exposed in the ENR
	TCPPort int
	// NetworkKey is the private key used to create the peer.ID if the node
	NetworkKey *ecdsa.PrivateKey
	// Bootnodes is a list of bootstrapper nodes
	Bootnodes []string
	// Log
	Logger *zap.Logger
}

// DefaultOptions returns the default options
func DefaultOptions(privateKey *ecdsa.PrivateKey) DiscV5Options {
	return DiscV5Options{
		NetworkKey: privateKey,
		Bootnodes:  make([]string, 0),
		Port:       commons.DefaultUDP,
		TCPPort:    commons.DefaultTCP,
		IP:         commons.DefaultIP,
		BindIP:     net.IPv4zero.String(),
	}
}

// Validate validates the options
func (opts *DiscV5Options) Validate() error {
	if opts.NetworkKey == nil {
		return errors.New("missing private key")
	}
	if opts.Port == 0 {
		return errors.New("missing udp port")
	}
	return nil
}

// IPs returns the external ip and bind ip
func (opts *DiscV5Options) IPs() (net.IP, net.IP, string) {
	ipAddr := net.ParseIP(opts.IP)
	if ipAddr == nil {
		ipAddr = net.ParseIP(commons.DefaultIP)
	}
	bindIP := net.ParseIP(opts.BindIP)
	if len(bindIP) == 0 {
		if ipAddr.To4() != nil {
			bindIP = net.IPv4zero
		} else {
			bindIP = net.IPv6zero
		}
	}
	n := "udp6"
	if bindIP.To4() != nil {
		n = "udp4"
	}
	return ipAddr, bindIP, n
}

// DiscV5Cfg creates discv5 config from the options
func (opts *DiscV5Options) DiscV5Cfg() (*discover.Config, error) {
	dv5Cfg := discover.Config{
		PrivateKey: opts.NetworkKey,
	}
	if len(opts.Bootnodes) > 0 {
		bootnodes, err := ParseENR(nil, false, opts.Bootnodes...)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse bootnodes ENR")
		}
		dv5Cfg.Bootnodes = bootnodes
	}

	if opts.Logger != nil {
		logger := log.New()
		logger.SetHandler(&dv5Logger{opts.Logger.With(zap.String("who", "dv5Logger"))})
		dv5Cfg.Log = logger
	}

	return &dv5Cfg, nil
}
