![Go](https://github.com/whitaker-io/machine/workflows/Go/badge.svg?branch=master)
[![GoDoc](https://godoc.org/github.com/whitaker-io/machine?status.svg)](https://godoc.org/github.com/whitaker-io/machine)
[![Go Report Card](https://goreportcard.com/badge/github.com/whitaker-io/machine)](https://goreportcard.com/report/github.com/whitaker-io/machine)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=whitaker-io/machine&amp;utm_campaign=Badge_Grade)
[![Codacy Badge](https://app.codacy.com/project/badge/Coverage/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&utm_medium=referral&utm_content=whitaker-io/machine&utm_campaign=Badge_Coverage)
[![Gitter chat](https://badges.gitter.im/whitaker-io/machine.png)](https://gitter.im/whitaker-io/machine)

`Machine` is a library for creating data workflows. These workflows can be either very concise or quite complex, even allowing for cycles for flows that need retry or self healing mechanisms.

### **Installation**


```bash
  go get -u github.com/whitaker-io/machine
```

***
## [Usage Overview](#usage-overview)

### **Usage**

Basic `receive` -> `process` -> `send` Flow

```golang
  m := NewStream("unique_id1", func(c context.Context) chan []Data {
    channel := make(chan []Data)
  
    //setup channel to collect data as long as the context has not completed

    return channel
  },
    &Option{FIFO: boolP(false)},
    &Option{Metrics: boolP(true)},
    &Option{Span: boolP(false)},
  )

  m.Builder().
    Map("unique_id2", func(m Data) error {
      var err error

      // ...do some processing

      return err
    }).
    Transmit("unique_id3", func(d []Data) error {
      // send a copy of the data somewhere

      return nil
    })

  if err := m.Run(context.Background()); err != nil {
    // Run will return an error in the case that one of the paths is not terminated
    panic(err)
  }
```

`Machine` can also duplicate the data and send it down multiple paths

```golang
  m := NewStream("unique_id1", func(c context.Context) chan []Data {
    channel := make(chan []Data)
  
    //setup channel to collect data as long as the context has not completed

    return channel
  },
    &Option{FIFO: boolP(false)},
    &Option{Metrics: boolP(true)},
    &Option{Span: boolP(false)},
  )

  left, right := m.Builder().Fork("unique_id2", ForkDuplicate)
  
  left.Transmit("unique_id3", func(d []Data) error {
    // send a copy of the data somewhere

    return nil
  })

  right.Transmit("unique_id4", func(d []Data) error {
    // send a copy of the data somewhere else

    return nil
  })

  if err := m.Run(context.Background()); err != nil {
    // Run will return an error in the case that one of the paths is not terminated
    panic(err)
  }
```

Incase of errors you can also route the errors down their own path with more complex flows having retry loops

```golang
  m := NewStream("unique_id1", func(c context.Context) chan []Data {
    channel := make(chan []Data)
  
    //setup channel to collect data as long as the context has not completed

    return channel
  },
    &Option{FIFO: boolP(false)},
    &Option{Metrics: boolP(true)},
    &Option{Span: boolP(false)},
  )

  left, right := m.Builder().Fork("unique_id2", ForkError)
  
  left.Transmit("unique_id3", func(d []Data) error {
    // send a copy of the data somewhere

    return nil
  })

  right.Transmit("unique_id4", func(d []Data) error {
    // send the error somewhere else

    return nil
  })

  if err := m.Run(context.Background()); err != nil {
    // Run will return an error in the case that one of the paths is not terminated
    panic(err)
  }
```

Routing based on filtering the data is also possible with ForkRule

```golang  
  m := NewStream("unique_id1", func(c context.Context) chan []Data {
    channel := make(chan []Data)
  
    //setup channel to collect data as long as the context has not completed

    return channel
  },
    &Option{FIFO: boolP(false)},
    &Option{Metrics: boolP(true)},
    &Option{Span: boolP(false)},
  )

  left, right := right.Fork("unique_id2", ForkRule(func(d Data) bool {
    //Data is a wrapper on typed.Typed see https://github.com/karlseguin/typed
    if val, ok := d["some_value"]; !ok {
      return false
    }

    return true
  }).Handler)
  
  left.Transmit("unique_id3", func(d []Data) error {
    // send a copy of the data somewhere

    return nil
  })

  right.Transmit("unique_id4", func(d []Data) error {
    // send the error somewhere else

    return nil
  })

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that one of the paths is not terminated
    panic(err)
  }
```

`machine` also supports FoldLeft and FoldRight operations 

```golang  
  m := NewStream("unique_id1", func(c context.Context) chan []Data {
    channel := make(chan []Data)
  
    //setup channel to collect data as long as the context has not completed

    return channel
  },
    &Option{FIFO: boolP(false)},
    &Option{Metrics: boolP(true)},
    &Option{Span: boolP(false)},
  ).FoldLeft("unique_id2", func(aggragate, value Data) Data {
    // do something to fold in the value or combine in some way

    // FoldLeft acts on the data from left to right

    // FoldRight acts on the data from right to left

    return aggragate
  }).Transmit("unique_id3", func(d []Data) error {
    // send a copy of the data somewhere

    return nil
  })

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that one of the paths is not terminated
    panic(err)
  }
```

using `Link` you can create loops to previous vertacies (use with care)

```golang  
  m := NewStream("machine_id", func(c context.Context) chan []Data {
    channel := make(chan []Data)
    go func() {
      for n := 0; n < count; n++ {
        channel <- testList
      }
    }()
    return channel
  },
    &Option{FIFO: boolP(false)},
    &Option{Idempotent: boolP(true)},
    &Option{Metrics: boolP(true)},
    &Option{Span: boolP(false)},
    &Option{BufferSize: intP(0)},
  )

  left, right := m.Builder().
    Map("unique_id2", func(m Data) error {
      var err error

      // ...do some processing

      return err
    }).
    Fork("fork_id", ForkRule(func(d Data) bool {
      // example counting logic for looping
      if val := typed.Typed(d).IntOr("__loops", 0); val > 4 {
        return true
      } else {
        d["__loops"] = val + 1
      }

      return false
    }).Handler)

  left.Transmit("sender_id", func(d []Data) error {
    out <- d
    return nil
  })

  right.Link("link_id", "map_id")

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that one of the paths is not terminated
    panic(err)
  }
```

***
## [License](#license)

Machine is provided under the [MIT License](https://github.com/whitaker-io/machine/blob/master/LICENSE).

```text
The MIT License (MIT)

Copyright (c) 2020 Jonathan Whitaker

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```