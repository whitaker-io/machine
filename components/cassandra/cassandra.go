package http

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gocql/gocql"
	"github.com/spf13/viper"
	"github.com/whitaker-io/machine"
)

// Initium func for providing a kafka based Initium
func Initium(v *viper.Viper) machine.Initium {
	hosts := v.GetStringSlice("hosts")
	keyspace := v.GetString("keyspace")
	interval := v.GetDuration("interval")
	query := v.GetString("query")
	pageSize := v.GetInt("page_size")

	channel := make(chan []map[string]interface{})

	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.Quorum
	session, _ := cluster.CreateSession()

	activeQuery := session.Query(query).PageSize(pageSize)

	return func(ctx context.Context) chan []map[string]interface{} {
		state := []byte{}
		activeQuery = activeQuery.WithContext(ctx)
		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					session.Close()
					break Loop
				case <-time.After(interval):
					iterator := activeQuery.PageState(state).Iter()

					if m, err := iterator.SliceMap(); err != nil {
						log.Printf("error querying data %v", err)
					} else {
						channel <- m
					}

					state = iterator.PageState()
				}
			}
		}()

		return channel
	}
}

// Terminus func for providing a kafka based Terminus
func Terminus(v *viper.Viper) machine.Terminus {
	hosts := v.GetStringSlice("hosts")
	keyspace := v.GetString("keyspace")
	query := v.GetString("query")
	keys := v.GetStringSlice("keys")

	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.Quorum
	session, _ := cluster.CreateSession()

	return func(m []map[string]interface{}) error {
		var errComposite error
		for _, packet := range m {
			values := []interface{}{}

			for _, key := range keys {
				values = append(values, packet[key])
			}

			if err := session.Query(query, values...).Exec(); err != nil {
				if errComposite == nil {
					errComposite = err
				} else {
					errComposite = fmt.Errorf("%v "+errComposite.Error(), err)
				}
			}
		}
		return errComposite
	}
}
