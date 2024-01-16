package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/fly-hiring/52397/proxy/pkg/app"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/exp/slog"
	"golang.org/x/sync/semaphore"
)

// workerPool is a pool of workers that handle connections.
type workerPool struct {
	// connectionsChan is a channel that receives connections from the server.
	connectionsChan <-chan *connection

	// logger is the logger used by the worker pool.
	logger *slog.Logger

	// activeConnections is a map that keeps track of the active connections.
	activeConnections map[net.Conn]struct{}
	mu                sync.Mutex

	// running is a flag that indicates if the worker pool is running.
	running bool

	// size is the size of the worker pool.
	// This is the maximum number of workers that can be running at the same time.
	// Limiting the number of workers is important because we don't want to
	// open too many connections and run out of file descriptors.
	size int

	// wg is a wait group that is used to wait for all connections to be handled.
	wg sync.WaitGroup
}

type connection struct {
	conn    net.Conn
	targets *app.Targets
}

func newWorkerPool(connectionsChan <-chan *connection, size int, logger *slog.Logger) *workerPool {
	return &workerPool{
		connectionsChan:   connectionsChan,
		size:              size,
		activeConnections: make(map[net.Conn]struct{}),
		logger:            logger,
	}
}

func (w *workerPool) run(ctx context.Context, errChan chan<- error) {
	var result error
	if w.running {
		w.logger.Error("worker pool already running", nil)
		errChan <- fmt.Errorf("worker pool already running")
		return
	}
	w.running = true
	// Create a semaphore to limit the number of workers
	sem := semaphore.NewWeighted(int64(w.size))
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case conn := <-w.connectionsChan:
			// acquire a semaphore
			// If we can't acquire the semaphore it means that either the context is canceled
			// or we have reached the timeout.
			// First case is handled by the select statement above.
			// Let's handle the second case. We close the connection and return an error.
			// The pool is busy and we can't handle the connection.
			if err := acquire(ctx, sem, 1); err != nil {
				w.logger.Error("acquiring semaphore", err)
				if err := conn.conn.Close(); err != nil {
					w.logger.Error("closing connection", err)
				}
				break
			}
			// Keep track of the active connections so we can close them
			w.addConnection(conn.conn)
			w.wg.Add(1)
			go func(conn *connection) {
				defer sem.Release(1)
				w.handleConnection(conn)
			}(conn)
		}
	}

	// out of the loop, either context is done or we got an error
	// let's close all the remaining connections
	if err := w.closeConnections(); err != nil {
		result = err
	}

	// Wait for all connections to be handled
	w.wg.Wait()
	errChan <- result
}

func (w *workerPool) handleConnection(conn *connection) {
	defer w.wg.Done()
	defer func() {
		w.closeConnection(conn.conn)
	}()

	// Get a target and make sure it's alive
	upstream, target, err := w.connectToValidUpstream(conn.targets)
	if err != nil {
		w.logger.Error("connecting to upstream", err)
		return
	}
	defer upstream.Close()

	go func() {
		_, err := io.Copy(upstream, conn.conn)
		if err != nil {
			w.logger.Error("copying data to upstream connection", err)
		}
	}()

	_, err = io.Copy(conn.conn, upstream)
	if err != nil {
		w.logger.Error("copying data to downstream connection", err)
	}
	w.logger.Info("closing connection", slog.String("target", target.Addr))
}

// connectToValidUpstream tries to connect to a valid upstream target.
func (w *workerPool) connectToValidUpstream(targets *app.Targets) (net.Conn, *app.Target, error) {
	var (
		upstream net.Conn
		target   *app.Target
		err      error
	)
	for i := 0; i < len(targets.Targets); i++ {
		target = getTarget(targets)
		// TODO: make the timeout configurable
		upstream, err = net.DialTimeout("tcp", target.Addr, 3*time.Minute)
		if err != nil {
			target.SetDead()
			continue
		}
		// get a tcpConn so we can set the keep alive
		tcpConn, ok := upstream.(*net.TCPConn)
		if !ok {
			return nil, nil, fmt.Errorf("could not convert to tcp connection")
		}
		// TODO: make the keep alive socket options configurable
		if err := tcpConn.SetKeepAlive(true); err != nil {
			return nil, nil, err
		}
		if err := tcpConn.SetKeepAlivePeriod(3 * time.Minute); err != nil {
			return nil, nil, err
		}
		return tcpConn, target, nil
	}
	return nil, nil, fmt.Errorf("could not connect to any upstream")
}

// perform a round robin on the targets and return the next target that is alive.
func getTarget(targets *app.Targets) *app.Target {
	var target *app.Target
	for {
		target = targets.RoundRobin()
		if !target.IsAlive() {
			continue
		}
		break
	}
	return target
}

func (w *workerPool) addConnection(conn net.Conn) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.activeConnections == nil {
		w.activeConnections = make(map[net.Conn]struct{})
	}
	w.activeConnections[conn] = struct{}{}
}

func (w *workerPool) closeConnection(conn net.Conn) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.activeConnections[conn]; ok {
		err := conn.Close()
		if err != nil {
			w.logger.Error("closing connection", err)
		}
		delete(w.activeConnections, conn)
	}
}

func (w *workerPool) closeConnections() error {
	var result error
	w.mu.Lock()
	defer w.mu.Unlock()
	for conn := range w.activeConnections {
		if err := conn.(*net.TCPConn).CloseRead(); err != nil {
			result = multierror.Append(result, err)
		}
		delete(w.activeConnections, conn)
	}
	return result
}

// acquire is a helper function that acquires n resources from the semaphore.
// It returns an error if the context is canceled or if the timeout is reached.
func acquire(ctx context.Context, sem *semaphore.Weighted, n int64) error {
	semCTX, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := sem.Acquire(semCTX, n); err != nil {
		return err
	}
	return nil
}
