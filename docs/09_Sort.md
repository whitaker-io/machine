The `Sort` method takes a 'Comparator' and applies a non-stable sort

`Sort`s method signature is `Sort`(id `string`, x `Comparator`) `Builder`

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
    ).
		Sort("sort_id", func(a, b data.Data) int {
      // do some comparison
			return 0
		}).
    Publish("publish_id", publishFN(func(d []data.Data) error {
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