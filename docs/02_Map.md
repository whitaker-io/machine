Using the `Map` method is how you can apply element-wise transformations or test for certain properties and accumulate errors on an element.


`Map`s method signature is `Map`(id `string`, x `Applicative`)

```golang
  stream := NewStream("unique_id1", 
    func(c context.Context) chan []Data {
      channel := make(chan []Data)
    
      // setup channel to collect data as long as 
      // the context has not completed

      return channel
    },
    &Option{FIFO: boolP(false)},
    &Option{Metrics: boolP(true)},
    &Option{Span: boolP(false)},
  )

  stream.Builder().Map("unique_id2", 
      func(m Data) error {
        var err error

        // ...do some processing

        return err
      },
    ).Publish("publish_left_id", publishFN(func(d []data.Data) error {
      // send the data somewhere

      return nil
    }),
  )

  if err := stream.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated (i.e. missing a Transmit)
    panic(err)
  }
```