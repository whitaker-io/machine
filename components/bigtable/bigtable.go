package bigtable

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/spf13/viper"
	"github.com/whitaker-io/machine"
)

type Filter func(r bigtable.Row) bool
type Mutation func(m []map[string]interface{}) (rowKeys []string, muts []*bigtable.Mutation)

// Initium func for providing a bigquery based Initium
func (f Filter) Initium(v *viper.Viper) machine.Initium {
	projectID := v.GetString("project_id")
	instance := v.GetString("instance")
	table := v.GetString("table")
	prefixRange := v.GetString("prefix_range")
	familyFilters := v.GetStringSlice("family_filters")
	interval := v.GetDuration("interval")

	client, err := bigtable.NewClient(context.Background(), projectID, instance)

	if err != nil {
		log.Fatalf("error connecting to bigquery %v", err)
	}

	tbl := client.Open(table)

	rr := bigtable.PrefixRange(prefixRange)

	filters := []bigtable.ReadOption{}
	for _, v := range familyFilters {
		filters = append(filters, bigtable.RowFilter(bigtable.FamilyFilter(v)))
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
					err := tbl.ReadRows(ctx, rr, func(r bigtable.Row) bool {
						if f(r) {
							m := map[string]interface{}{
								"__key": r.Key(),
							}

							for k, v := range r {
								m[k] = v
							}

							payload = append(payload)
							return true
						}
						return false
					}, filters...)
					if err != nil {
						log.Printf("error reading from bigtable %v", err)
					}

					channel <- payload
				}
			}
		}()
		return channel
	}
}

// Terminus func for providing a bigquery based Terminus
func (muter Mutation) Terminus(v *viper.Viper) machine.Terminus {
	projectID := v.GetString("project_id")
	instance := v.GetString("instance")
	table := v.GetString("table")

	client, err := bigtable.NewClient(context.Background(), projectID, instance)

	if err != nil {
		log.Fatalf("error connecting to bigquery %v", err)
	}

	tbl := client.Open(table)

	return func(m []map[string]interface{}) error {
		var errComposite error

		keys, muts := muter(m)

		if errs, err := tbl.ApplyBulk(context.Background(), keys, muts); err != nil || len(errs) > 0 {
			for _, e := range errs {
				err = fmt.Errorf("%w "+err.Error(), e)
			}
			if errComposite == nil {
				errComposite = err
			} else {
				errComposite = fmt.Errorf("%v "+errComposite.Error(), err)
			}
		}
		return errComposite
	}
}
