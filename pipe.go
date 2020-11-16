package machine

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
	port       string
	app        *fiber.App
	streams    map[string]Stream
	healthInfo map[string]*HealthInfo
	logStore   LogStore
	logger     Logger
}

// HealthInfo type for giving info on the stream
type HealthInfo struct {
	StreamID    string     `json:"stream_id"`
	LastPayload time.Time  `json:"last_payload"`
	mtx         sync.Mutex `json:"-"`
}

// Run func to start the server
func (pipe *Pipe) Run(ctx context.Context, gracePeriod time.Duration) error {
	if len(pipe.streams) < 1 {
		return fmt.Errorf("no streams found")
	}

	streamIDs := []string{}
	for key, stream := range pipe.streams {
		streamID := key
		streamIDs = append(streamIDs, streamID)

		recorder := func(vertexID, vertexType, state string, payload []*Packet) {
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
		if err := stream.Run(ctx, recorder); err != nil {
			return err
		}
	}

	err := pipe.logStore.Join(pipe.id,
		func(logs ...*Log) {
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
		},
		streamIDs...,
	)

	if err != nil {
		return err
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

	return pipe.app.Listen(pipe.port)
}

// StreamHTTP func for creating a Stream at the path /stream/<id>
func (pipe *Pipe) StreamHTTP(id string, opts ...*Option) Stream {
	channel := make(chan []Data)

	pipe.app.Add(http.MethodPost, "/stream/"+id, func(ctx *fiber.Ctx) error {
		payload := []Data{}
		packet := Data{}

		if body := string(ctx.Body()); len(body) < 4 {
			return ctx.SendStatus(http.StatusBadRequest)
		} else if []rune(body)[0] == rune('{') {
			if ctx.BodyParser(packet) != nil {
				return ctx.SendStatus(http.StatusBadRequest)
			}

			payload = []Data{packet}
		} else if err := ctx.BodyParser(payload); err != nil {
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

		channel <- payload

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

	return pipe.streams[id]
}

// StreamSubscription func for creating a Stream at the that reads from a subscription
func (pipe *Pipe) StreamSubscription(id string, sub Subscription, interval time.Duration, opts ...*Option) Stream {
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

	return pipe.streams[id]
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

// NewPipe func for creating a new server instance
func NewPipe(port string, logger Logger, store LogStore, config ...fiber.Config) *Pipe {
	pipe := &Pipe{
		id:         uuid.New().String(),
		app:        fiber.New(config...),
		port:       port,
		streams:    map[string]Stream{},
		healthInfo: map[string]*HealthInfo{},
		logStore:   store,
		logger:     logger,
	}

	pipe.app.Add(http.MethodGet, "/health", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(map[string]interface{}{
			"pipe_id":     pipe.id,
			"health_info": pipe.healthInfo,
		})
	})

	return pipe
}
