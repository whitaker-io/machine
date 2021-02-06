The `Loop` method allows the stream to create `while`/`for` loops with the fork provided being the check for the loop

`Loop`s method signature is `Loop`(id `string`, x `Fork`, options ...*`Option`) (loop `LoopBuilder`, out `Builder`)

It returns a `LoopBuilder` which is used for building the logic inside the loop and a `Builder` which is the logic after/outside the loop

Nested loops are supported and `LoopBuilder` supports all of the methods of `Builder` except for `Link`. This
is so `goto` patterns which can be extremely difficult to debug are not implemented.

`LoopBuilder` also has a `Done` method which is required to be called in order to close the loop

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

  loop, out := m.Builder().
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
    ).Done()

  out.Transmit("sender_id", 
    func(d []Data) error {
      out <- d
      return nil
    },
  )

  

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that
    // one of the paths is not terminated
    panic(err)
  }
```