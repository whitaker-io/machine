package bigquery

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/spf13/viper"
	"github.com/whitaker-io/machine"
	"google.golang.org/api/iterator"
)

type loader map[string]interface{}

func (l loader) Load(v []bigquery.Value, s bigquery.Schema) error {
	for i := 0; i < len(s); i++ {
		l[s[i].Name] = v[i]
	}
	return nil
}

func (l loader) Save() (row map[string]bigquery.Value, id string, err error) {
	row = map[string]bigquery.Value{}

	for k, v := range l {
		row[k] = v
	}

	return row, "", nil
}

// Initium func for providing a bigquery based Initium
func Initium(v *viper.Viper) machine.Initium {
	projectID := v.GetString("project_id")
	query := v.GetString("query")
	interval := v.GetDuration("interval")

	client, err := bigquery.NewClient(context.Background(), projectID)
	if err != nil {
		log.Fatalf("error connecting to bigquery %v", err)
	}
	channel := make(chan []map[string]interface{})
	return func(ctx context.Context) chan []map[string]interface{} {
		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					break Loop
				case <-time.After(interval):
					payload := []map[string]interface{}{}
					q := client.Query(query)
					it, err := q.Read(ctx)
					if err != nil {
						log.Printf("error reading from bigquery %v", err)
					}

					for {
						value := loader{}
						err := it.Next(&value)
						if err == iterator.Done {
							break
						} else if err != nil {
							log.Printf("error reading from bigquery iterator %v", err)
						} else {
							payload = append(payload, value)
						}
					}

					channel <- payload
				}
			}
		}()
		return channel
	}
}

// Terminus func for providing a bigquery based Terminus
func Terminus(v *viper.Viper) machine.Terminus {
	projectID := v.GetString("project_id")
	datasetName := v.GetString("dataset")
	tableName := v.GetString("table")

	client, err := bigquery.NewClient(context.Background(), projectID)
	if err != nil {
		log.Fatalf("error connecting to bigquery %v", err)
	}

	table := client.Dataset(datasetName).Table(tableName)

	return func(m []map[string]interface{}) error {
		var errComposite error
		for _, row := range m {
			if err := table.Inserter().Put(context.Background(), loader(row)); err != nil {
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
