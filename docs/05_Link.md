The `Link` method allows the stream to send data back to a previous vertex in the stream. This is how you can accomplish retries in case of failures, and other loop based operations

`Link`'s method signature is thus slightly different and instead of a function as a parameter takes an ID associated with an ancestor vertex in the stream.


```golang  
  m := NewStream("machine_id", 
    func(c context.Context) chan []Data {
      channel := make(chan []Data)
    
      // setup channel to collect data as long as
      // the context has not completed
      
      return channel
    },
    &Option{FIFO: boolP(false)},
    &Option{Idempotent: boolP(true)},
    &Option{Metrics: boolP(true)},
    &Option{Span: boolP(false)},
    &Option{BufferSize: intP(0)},
  )

  left, right := m.Builder().
    Map("unique_id2", 
      func(m Data) error {
        var err error

        // ...do some processing

        return err
      },
    ).Fork("fork_id", 
    ForkRule(func(d Data) bool {
        // example counting logic for looping
        if val := typed.Typed(d).IntOr("__loops", 0); val > 4 {
          return true
        } else {
          d["__loops"] = val + 1
        }

        return false
      }).Handler,
    )

  left.Transmit("sender_id", 
    func(d []Data) error {
      out <- d
      return nil
    },
  )

  right.Link("link_id", "map_id")

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that
    // one of the paths is not terminated
    panic(err)
  }
```