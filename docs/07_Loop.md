The `Loop` method allows the stream to create `while`/`for` loops with the fork provided being the check for the loop

`Loop`s method signature is `Loop`(id `string`, x `Fork`) (loop `Builder`, out `Builder`)

It returns 2 `Builder`s where the first is for the loop logic and the second is for leaving the loop


```golang  
  stream := NewStream("machine_id", 
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

  loop, out := stream.Builder().
    Loop("loop_id", 
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

  loop.Map("unique_id2", 
    func(m Data) error {
      var err error

      // ...do some processing

      return err
    },
  )

  out.Publish("publish_id", publishFN(func(d []data.Data) error {
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