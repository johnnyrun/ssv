package streams

import (
	"context"
	"github.com/bloxapp/ssv/network/forks"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

// StreamResponder abstracts the stream access with a simpler interface that accepts only the data to send
type StreamResponder func([]byte) error

// StreamController simplifies the interaction with libp2p streams.
type StreamController interface {
	// Request sends a message to the given stream and returns the response
	Request(peerID peer.ID, protocol protocol.ID, msg []byte) ([]byte, error)
	// HandleStream is called at the beginning of stream handlers to create a wrapper stream and read first message
	HandleStream(stream core.Stream) ([]byte, StreamResponder, func(), error)
}

// NewStreamController create a new instance of StreamController
func NewStreamController(ctx context.Context, logger *zap.Logger, host host.Host,
	fork forks.Fork, requestTimeout time.Duration) StreamController {
	ctrl := streamCtrl{
		ctx:            ctx,
		logger:         logger,
		host:           host,
		fork:           fork,
		requestTimeout: requestTimeout,
	}

	return &ctrl
}

type streamCtrl struct {
	ctx context.Context

	logger *zap.Logger

	host host.Host
	fork forks.Fork

	requestTimeout time.Duration
}

// Request sends a message to the given stream and returns the response
func (n *streamCtrl) Request(peerID peer.ID, protocol protocol.ID, data []byte) ([]byte, error) {
	s, err := n.host.NewStream(n.ctx, peerID, protocol)
	if err != nil {
		return nil, err
	}
	stream := NewStream(s)
	defer func() {
		if err := stream.Close(); err != nil {
			n.logger.Error("could not close stream", zap.Error(err))
		}
	}()
	metricsStreamRequests.Inc()
	metricsStreamRequestsActive.Inc()
	defer metricsStreamRequestsActive.Dec()

	if err := stream.WriteWithTimeout(data, n.requestTimeout); err != nil {
		return nil, errors.Wrap(err, "could not write to stream")
	}
	if err := s.CloseWrite(); err != nil {
		return nil, errors.Wrap(err, "could not close write stream")
	}
	res, err := stream.ReadWithTimeout(n.requestTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "could not read stream msg")
	}
	metricsStreamRequestsSuccess.Inc()
	return res, nil
}

// HandleStream is called at the beginning of stream handlers to create a wrapper stream and read first message
// it returns functions to respond and close the stream
func (n *streamCtrl) HandleStream(stream core.Stream) ([]byte, StreamResponder, func(), error) {
	s := NewStream(stream)

	streamID := s.ID()
	done := func() {
		if err := s.Close(); err != nil {
			n.logger.Error("could not close stream", zap.String("streamID", streamID), zap.Error(err))
		}
	}
	data, err := s.ReadWithTimeout(n.requestTimeout)
	if err != nil {
		return nil, nil, done, errors.Wrap(err, "could not read stream msg")
	}

	return data, func(res []byte) error {
		if err := s.WriteWithTimeout(res, n.requestTimeout); err != nil {
			return errors.Wrap(err, "could not write to stream")
		}
		metricsStreamResponses.Inc()
		return nil
	}, done, nil
}
