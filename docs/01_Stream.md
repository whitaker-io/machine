`Stream` is the the type used for interacting controlling the flow of the data. It has the following functions:

  * `ID`() string
  * `Run`(ctx context.Context, recorders ...func(id, type, state string, payload []*Packet)) error
  * `Inject`(ctx context.Context, events map[string][]*Packet)
  * `Builder`() Builder 

------

`Run` is the method used to start the processing. It takes a context.Context which is used to control the lifespan of the processing. It also takes a variadic of 

```golang
func(id, type, state string, payload []*Packet)
```
going forward these will be referred to as recorders.

Recorders are used for communicating state outside of the stream. This is how clusting takes place. More on this later.

------

`Inject` is a method used for side loading data into the stream. The use case for this is if a node in a clustered environment when down and the work needs to be picked up where it left off.

It takes a context.Context and a map[string][]*Packet. The keys of the map are the ID's of the vertices meant to send the payload to.

------

`Builder` is the entry point to `Stream` creation.  

------

Basic `receive` -> `process` -> `send` Flow

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