Using the `Fork` method allows you to split data into multiple channels.

`Machine` can duplicate the data and send it down multiple paths

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
  )

  left, right := m.Builder().Fork("unique_id2", ForkDuplicate)
  
  left.Transmit("unique_id3", 
    func(d []Data) error {
      // send a copy of the data somewhere

      return nil
    },
  )

  right.Transmit("unique_id4", 
    func(d []Data) error {
      // send a copy of the data somewhere else

      return nil
    },
  )

  if err := m.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated
    panic(err)
  }
```

Incase of errors you can also route the errors down their own path with more complex flows having retry loops uning `Link`

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
  )

  left, right := m.Builder().Fork("unique_id2", ForkError)
  
  left.Transmit("unique_id3", 
    func(d []Data) error {
      // send a copy of the data somewhere

      return nil
    },


  right.Transmit("unique_id4", 
    func(d []Data) error {
      // send the error somewhere else

      return nil
    },
  )

  if err := m.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated
    panic(err)
  }
```


Routing based on filtering the data is also possible with ForkRule

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
  )

  left, right := right.Fork("unique_id2", 
    ForkRule(func(d Data) bool {
      if val, ok := d["some_value"]; !ok {
        return false
      }

      return true
    }).Handler,
  )
  
  left.Transmit("unique_id3", 
    func(d []Data) error {
      // send a copy of the data somewhere

      return nil
    },
  )

  right.Transmit("unique_id4", 
    func(d []Data) error {
      // send the error somewhere else

      return nil
    },
  )

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated
    panic(err)
  }
```