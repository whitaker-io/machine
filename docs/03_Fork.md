Using the `Fork` method allows you to split data into multiple channels.

`Fork`s method signature is `Fork`(id `string`, x `Fork`)

`Machine` has a number of built-in `Fork`'s

 * ForkDuplicate to send it down multiple paths

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

  left, right := stream.Builder().Fork("unique_id2", ForkDuplicate)
  
  left.Publish("publish_left_id", publishFN(func(d []data.Data) error {
      // send the data somewhere

      return nil
    }),
  )

  right.Publish("publish_right_id", publishFN(func(d []data.Data) error {
      // send the data somewhere else

      return nil
    }),
  )

  if err := stream.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated
    panic(err)
  }
```

 * ForkError to filter out errors

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

  left, right := stream.Builder().Fork("unique_id2", ForkError)
  
  left.Publish("publish_left_id", publishFN(func(d []data.Data) error {
      // send the data somewhere

      return nil
    }),
  )

  right.Publish("publish_right_id", publishFN(func(d []data.Data) error {
      // send the data somewhere else

      return nil
    }),
  )

  if err := stream.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated
    panic(err)
  }
```

 * ForkRule to wrap a `func`(d `Data`) `bool` and handle the split

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

  left, right := stream.Builder().Fork("unique_id2", 
    ForkRule(func(d Data) bool {
      if val, ok := d["some_value"]; !ok {
        return false
      }

      return true
    }).Handler,
  )

  left.Publish("publish_left_id", publishFN(func(d []data.Data) error {
      // send the data somewhere

      return nil
    }),
  )

  right.Publish("publish_right_id", publishFN(func(d []data.Data) error {
      // send the data somewhere else

      return nil
    }),
  )

  if err := stream.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated
    panic(err)
  }
```