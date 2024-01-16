package server

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/fly-hiring/52397/proxy/pkg/app"
	"golang.org/x/exp/slog"
)

type Server struct {
	// connectionsChan is a channel that receives connections from the server.
	// The worker pool will read from this channel and handle the connections
	// in a separate goroutines.
	connectionsChan chan *connection

	// logger is the logger used by the server.
	logger *slog.Logger

	// poolErr is a channel that receives errors from the worker pool.
	poolErr chan error

	// close is a channel that is used to close the server.
	// When a signal is received on this channel, the server will close all listeners.
	close chan struct{}

	// listeners is a list of listeners that are used to accept connections.
	// We need to keep track of these listeners so we can gracefully close them.
	listeners []net.Listener

	// mu is a mutex that is used to protect the listeners list.
	mu sync.Mutex

	// workerPool is a pool of workers that handle connections.
	workerPool *workerPool

	// Apps is a list of applications that the server is proxying.
	Apps *app.Apps

	// wg is a wait group that is used to wait for all listeners to close.
	wg sync.WaitGroup
}

// NewServer creates a new server.
func NewServer(apps *app.Apps, size int, logger *slog.Logger) *Server {
	// make connection channel
	// size*2 because we want to be able to accept connections while the workers are
	// handling the existing connections. But we have to timeout the connections so we don't
	// keep them open forever.
	connectionsChan := make(chan *connection, size*2)
	// Create the worker pool
	workerPool := newWorkerPool(connectionsChan, size, logger)
	return &Server{
		Apps:            apps,
		workerPool:      workerPool,
		connectionsChan: connectionsChan,
		poolErr:         make(chan error),
		close:           make(chan struct{}),
		logger:          logger,
	}
}

// Start starts the server.
func (s *Server) Start(ctx context.Context) {
	// Start the worker pool
	go s.workerPool.run(ctx, s.poolErr)

	// Start the server
	for _, app := range s.Apps.Apps {
		for _, port := range app.Ports {
			s.wg.Add(1)
			go s.run(port, app.Targets, app.Name)
		}
	}
}

func (s *Server) run(port int, targets *app.Targets, name string) {
	defer s.wg.Done()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		s.logger.Error("error listening on port", err)
		return
	}
	// keep track of the listener so we can close it later
	s.addListener(ln)
	s.logger.Info("starting to listen on", slog.Int("port", port), slog.String("app", name))
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.close:
				// closing the server, just return
				return
			default:
				// just log the error and move on
				s.logger.Error("error accepting connection", err)
			}
		} else {
			// push to a channel to let the worker pool handle it
			s.connectionsChan <- &connection{
				conn:    conn,
				targets: targets,
			}
		}
	}
}

func (s *Server) addListener(ln net.Listener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, ln)
}

func (s *Server) Shutdown(cancel context.CancelFunc) {
	s.logger.Info("server stopping")

	// stop accepting new connections
	close(s.close)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ln := range s.listeners {
		ln.Close()
	}
	s.wg.Wait()

	// cancel the context, so the worker pool stops
	cancel()
	err := <-s.poolErr
	if err != nil {
		s.logger.Error("while stopping worker pool", err)
	}

	// Wait for the server to stop
	s.logger.Info("server stopped")
}
