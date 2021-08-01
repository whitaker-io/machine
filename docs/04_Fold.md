The `Fold` methods are used for combining the elements of the payload based on a provided accumulator. 

There are 2 Fold methods available; 

  * `FoldLeft` which is applies the accumulator from left to right 

`FoldLeft`s method signature is `FoldLeft`(id `string`, x `Fold`)
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
  ).FoldLeft("unique_id2", 
    func(aggragate, value Data) Data {
      // do something to fold in the value or combine
      // in some way

      // FoldLeft acts on the data from left to right

      return aggragate
    },
  ).Publish("publish_id", publishFN(func(d []data.Data) error {
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

  * `FoldRight` which oppositely applies from right to left

`FoldRight`s method signature is `FoldRight`(id `string`, x `Fold`)
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
  ).FoldRight("unique_id2", 
    func(aggragate, value Data) Data {
      // do something to fold in the value or combine
      // in some way

      // FoldRight acts on the data from right to left

      return aggragate
    },
  ).Publish("publish_id", publishFN(func(d []data.Data) error {
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