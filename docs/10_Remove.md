The `Remove` method allows the stream discard some of the data

`Remove`s method signature is `Remove`(id `string`, x `Remover`) `Builder`

```golang  
  stream := NewStream("machine_id", 
    func(c context.Context) chan []data.Data {
      channel := make(chan []data.Data)
      go func() {
        for n := 0; n < count; n++ {
          channel <- deepCopy(testListBase)
        }
      }()
      return channel
    },
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	stream.Builder().Map("map_id", 
      func(m data.Data) error {
        // process the data

        return nil
      },
    ).Remove("remove_id", 
      func(index int, d data.Data) bool {
        // decide what to remove

        return false
      },
    ).Publish("publish_id",
      publishFN(func(d []data.Data) error {

        // send the data somewhere

        return nil
      }),
    )

  

  if err := stream.Run(context.Background()); err != nil {
    // Run will return an error in the case that
    // one of the paths is not terminated
    panic(err)
  }
```