Using the `Map` method is how you can apply element-wise transformations or test for certain properties and accumulate errors on an element.

`Map` takes a unique ID (used for injection), a machine.Applicative (`func`(m machine.Data) error), and a variadic of override machine.Option settings

Considering the basic example:


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

  m.Builder().
    Map("unique_id2", 
      func(m Data) error {
        var err error

        // ...do some processing

        return err
      },
    ).
    Transmit("unique_id3", 
      func(d []Data) error {
        // send a copy of the data somewhere

        return nil
      },
    )

  if err := m.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated (i.e. missing a Transmit)
    panic(err)
  }
```

You could contact other services to enrich the data, check to make sure the required information is required, or add new fields