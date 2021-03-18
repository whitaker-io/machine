`Publish` is possibly the most important part of the `Stream` in that it is the outlet of data. It is important to note that `error`'s returned here are logged only.

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
    Publish("unique_id3", publisher)

  if err := m.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated (i.e. missing a Publish)
    panic(err)
  }
```