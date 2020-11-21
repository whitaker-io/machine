The `Fold` methods are used for combining the elements of the payload based on a provided accumulator. 

There are 2 Fold methods available; 

  * `FoldLeft` which is applies the accumulator from left to right 
```golang  
  m := NewStream("unique_id1", 
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
  ).Transmit("unique_id3", 
    func(d []Data) error {
      // send a copy of the data somewhere

      return nil
    },
  )

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that
    // one of the paths is not terminated
    panic(err)
  }
```

  * `FoldRight` which oppositely applies from right to left

```golang  
  m := NewStream("unique_id1", 
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
  ).Transmit("unique_id3", 
    func(d []Data) error {
      // send a copy of the data somewhere

      return nil
    },
  )

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that
    // one of the paths is not terminated
    panic(err)
  }
```