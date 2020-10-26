package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/spf13/viper"
	"github.com/whitaker-io/machine"
)

// Initium func for providing a fiber.Handler based Initium
func Initium(v *viper.Viper) machine.Initium {
	serverName := v.GetString("name")
	port := v.GetString("port")
	path := v.GetString("path")
	bodyLimit := v.GetInt("body_limit")
	readBufferSize := viper.GetInt("read_buffer_size")
	readTimeout := v.GetDuration("read_timeout")
	writeBufferSize := viper.GetInt("write_buffer_size")
	writeTimeout := v.GetDuration("write_timeout")

	channel := make(chan []map[string]interface{})

	s := fiber.New(fiber.Config{
		DisableKeepalive: true,
		BodyLimit:        bodyLimit,
		ReadBufferSize:   readBufferSize,
		ReadTimeout:      readTimeout,
		ServerHeader:     serverName,
		WriteBufferSize:  writeBufferSize,
		WriteTimeout:     writeTimeout,
	})

	s.Add(http.MethodPost, path, func(ctx *fiber.Ctx) error {
		payload := []map[string]interface{}{}

		if err := ctx.BodyParser(payload); err != nil {
			return ctx.SendStatus(http.StatusBadRequest)
		}

		channel <- payload

		return ctx.SendStatus(http.StatusOK)
	})

	return func(ctx context.Context) chan []map[string]interface{} {
		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					if err := s.Shutdown(); err != nil {
						log.Printf("error shutting down %v", err)
					}
					break Loop
				case <-time.After(5 * time.Second):
				}
			}
		}()

		if err := s.Listen(port); err != nil {
			log.Fatalf("error starting server %v", err)
		}

		return channel
	}
}

// Terminus func for providing a http client based Terminus
func Terminus(v *viper.Viper) machine.Terminus {
	host := v.GetString("host")
	timeout := v.GetDuration("timeout")

	client := http.Client{
		Timeout: timeout,
	}

	return func(m []map[string]interface{}) error {
		bytez, err := json.Marshal(m)

		if err != nil {
			return err
		}

		resp, err := client.Post(host, "application/json", bytes.NewReader(bytez))

		if e := resp.Body.Close(); e != nil {
			return e
		}

		if err != nil {
			return err
		}

		if resp.StatusCode > 299 {
			return fmt.Errorf("error sending payload to server %s - response code %d", host, resp.StatusCode)
		}

		return nil
	}
}
