`Stream` is the the type used for interacting controlling the flow of the data. It has the following functions:

  * `ID`() `string`
  * `Run`(ctx `context`.`Context`) `error`
  * `Inject`(id `string`, payload ...*`Packet`)
	* `VertexIDs`() []`string`
  * `Builder`() `Builder`
  * `Errors`() `chan` `error`

Basic `receive` -> `process` -> `send` Flow

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