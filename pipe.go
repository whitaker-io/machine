package machine

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Subscription interface for creating a pull based stream
type Subscription interface {
	Read(ctx context.Context) []Data
	Close() error
}

// Logger type for accepting log messages
type Logger interface {
	Error(...interface{})
	Info(...interface{})
}

// LogStore type for managing cluster state
type LogStore interface {
	Join(id string, callback InjectionCallback, streamIDs ...string) error
	Write(logs ...*Log)
	Leave(id string) error
}

// InjectionCallback func to run when the LogStore decides to restart the flow of orphaned Data
type InjectionCallback func(logs ...*Log)

// Log type for holding the data that is recorded from the streams
type Log struct {
	OwnerID    string    `json:"owner_id"`
	StreamID   string    `json:"stream_id"`
	VertexID   string    `json:"vertex_id"`
	VertexType string    `json:"vertex_type"`
	State      string    `json:"state"`
	Packet     *Packet   `json:"packet"`
	When       time.Time `json:"when"`
}

// Pipe type for holding the server information for running http servers
type Pipe struct {
	id         string
	app        *fiber.App
	streams    map[string]Stream
	healthInfo map[string]*HealthInfo
	logStore   LogStore
	logger     Logger
}

// HealthInfo type for giving info on the stream
type HealthInfo struct {
	StreamID    string    `json:"stream_id"`
	LastPayload time.Time `json:"last_payload"`
	mtx         sync.Mutex
}

// Run func to start the server
func (pipe *Pipe) Run(ctx context.Context, port string, gracePeriod time.Duration) error {
	if len(pipe.streams) < 1 {
		return fmt.Errorf("no streams found")
	}

	streamIDs := []string{}
	for key := range pipe.streams {
		streamIDs = append(streamIDs, key)
	}

	if err := pipe.logStore.Join(pipe.id, pipe.injectionCallback(ctx), streamIDs...); err != nil {
		return err
	}

	for key, stream := range pipe.streams {
		if err := stream.Run(ctx, pipe.recorder(key)); err != nil {
			return err
		}
	}

	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				if err := pipe.logStore.Leave(pipe.id); err != nil {
					pipe.logger.Error(err)
				}
				if err := pipe.app.Shutdown(); err != nil {
					pipe.logger.Error(err)
				}
				break Loop
			default:
				<-time.After(gracePeriod)
			}
		}
	}()

	return pipe.app.Listen(port)
}

// Stream func for registering a Stream with the Pipe
func (pipe *Pipe) Stream(stream Stream) Builder {
	id := stream.ID()

	pipe.streams[id] = stream

	pipe.healthInfo[id] = &HealthInfo{
		StreamID: id,
	}

	return pipe.streams[id].Builder()
}

// StreamHTTP func for creating a Stream at the path /stream/<id>
func (pipe *Pipe) StreamHTTP(id string, opts ...*Option) Builder {
	channel := make(chan []Data)

	pipe.app.Post("/stream/"+id, func(ctx *fiber.Ctx) error {
		payload := []Data{}
		packet := Data{}

		if err := ctx.BodyParser(&packet); err == nil {
			payload = []Data{packet}
		} else if err := ctx.BodyParser(&payload); err != nil {
			return ctx.SendStatus(http.StatusBadRequest)
		}

		now := time.Now()
		go func() {
			pipe.healthInfo[id].mtx.Lock()
			defer pipe.healthInfo[id].mtx.Unlock()
			if now.After(pipe.healthInfo[id].LastPayload) {
				pipe.healthInfo[id].LastPayload = now
			}
		}()

		channel <- deepCopy(payload)

		return ctx.SendStatus(http.StatusAccepted)
	})

	pipe.streams[id] = NewStream(id,
		func(ctx context.Context) chan []Data {
			return channel
		},
		opts...,
	)

	pipe.healthInfo[id] = &HealthInfo{
		StreamID: id,
	}

	return pipe.streams[id].Builder()
}

// StreamSubscription func for creating a Stream at the that reads from a subscription
func (pipe *Pipe) StreamSubscription(id string, sub Subscription, interval time.Duration, opts ...*Option) Builder {
	channel := make(chan []Data)

	pipe.streams[id] = NewStream(id,
		func(ctx context.Context) chan []Data {
			go func() {
			Loop:
				for {
					select {
					case <-ctx.Done():
						if err := sub.Close(); err != nil {
							pipe.logger.Error(map[string]interface{}{
								"subscription_id": id,
								"message":         "error closing subsciption",
								"error":           err,
							})
						}
						break Loop
					case <-time.After(interval):
						now := time.Now()
						go func() {
							pipe.healthInfo[id].mtx.Lock()
							defer pipe.healthInfo[id].mtx.Unlock()
							if now.After(pipe.healthInfo[id].LastPayload) {
								pipe.healthInfo[id].LastPayload = now
							}
						}()

						channel <- sub.Read(ctx)
					}
				}
			}()

			return channel
		},
		opts...,
	)

	pipe.healthInfo[id] = &HealthInfo{
		StreamID: id,
	}

	return pipe.streams[id].Builder()
}

// Use Wraps fiber.App.Use
//
// Use registers a middleware route that will match requests with the provided prefix (which is optional and defaults to "/").
//
//   app.Use(func(c *fiber.Ctx) error {
//      return c.Next()
//   })
//   app.Use("/api", func(c *fiber.Ctx) error {
//      return c.Next()
//   })
//   app.Use("/api", handler, func(c *fiber.Ctx) error {
//      return c.Next()
//   })
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
func (pipe *Pipe) Use(args ...interface{}) {
	pipe.app.Use(args...)
}

func (pipe *Pipe) recorder(streamID string) recorder {
	return func(vertexID, vertexType, state string, payload []*Packet) {
		logs := make([]*Log, len(payload))
		now := time.Now()
		for i, packet := range payload {
			logs[i] = &Log{
				OwnerID:    pipe.id,
				StreamID:   streamID,
				VertexID:   vertexID,
				VertexType: vertexType,
				State:      state,
				Packet:     packet,
				When:       now,
			}
		}
		if pipe.logger != nil {
			pipe.logger.Info(logs)
		}
		if pipe.logStore != nil {
			pipe.logStore.Write(logs...)
		}
	}
}

func (pipe *Pipe) injectionCallback(ctx context.Context) func(logs ...*Log) {
	return func(logs ...*Log) {
		for _, log := range logs {
			if stream, ok := pipe.streams[log.StreamID]; ok {
				stream.Inject(ctx, map[string][]*Packet{
					log.VertexID: {log.Packet},
				})
			} else {
				pipe.logger.Error(map[string]interface{}{
					"message": "unknown stream",
					"log":     log,
				})
			}
		}
	}
}

// NewPipe func for creating a new server instance
func NewPipe(id string, logger Logger, store LogStore, config ...fiber.Config) *Pipe {
	pipe := &Pipe{
		id:         id,
		app:        fiber.New(config...),
		streams:    map[string]Stream{},
		healthInfo: map[string]*HealthInfo{},
		logStore:   store,
		logger:     logger,
	}

	pipe.Use(recover.New())

	pipe.app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(map[string]interface{}{
			"pipe_id":     pipe.id,
			"health_info": pipe.healthInfo,
		})
	})

	return pipe
}
