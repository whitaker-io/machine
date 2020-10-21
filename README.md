![Go](https://github.com/whitaker-io/machine/workflows/Go/badge.svg?branch=master)
[![GoDoc](https://godoc.org/github.com/whitaker-io/machine?status.svg)](https://godoc.org/github.com/whitaker-io/machine)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=whitaker-io/machine&amp;utm_campaign=Badge_Grade)
[![Codacy Badge](https://app.codacy.com/project/badge/Coverage/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&utm_medium=referral&utm_content=whitaker-io/machine&utm_campaign=Badge_Coverage)
[![Gitter chat](https://badges.gitter.im/whitaker-io/machine.png)](https://gitter.im/gitterHQ/gitter)

`Machine` is a library for creating data workflows. These workflows can be either very concise or quite complex, even allowing for cycles for flows that need retry or self healing mechanisms.

***
## [Usage Overview](#usage-overview)

### **Usage**

Basic `receive` -> `process` -> `send` Flow

```golang
  // fifo controls whether or not the data is processed in order of receipt
  fifo := false
  // bufferSize controls the go channel buffer size, set to 0 for unbuffered channels
  bufferSize := 0

  machineInstance := New(uuid.New().String(), "machine_name", fifo, func(c context.Context) chan []map[string]interface{} {
    return channel
  }).Then(
    NewVertex(uuid.New().String(), "unique_vertex_name", fifo, func(m map[string]interface{}) error {
      
      // ...do some processing
      
      return nil
    }).
    Terminate(
      NewTermination(uuid.New().String(), "unique_termination_name", !fifo, func(list []map[string]interface{}) error {

        // send the data somewhere
        
        return nil
      }),
    ),

  // Build take an int buffer size and some Recorder functions to allow for state management or logging.
  // Having a Recorder requires the use of a Deep Copy operation which can be expensive depending on
  // the data being processed
  ).Build(bufferSize, func(vertexID, operation string, payload []*Packet) {})

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that one of the paths is not terminated
    panic(err)
  }
```

`Machine` can also duplicate the data and send it down multiple paths

```golang
  // fifo controls whether or not the data is processed in order of reciept
  fifo := false
  // bufferSize controls the go channel buffer size, set to 0 for unbuffered channels
  bufferSize := 0

  machineInstance := New(uuid.New().String(), "machine_name", fifo, func(c context.Context) chan []map[string]interface{} {
    return channel
  }).Route(
    NewRouter(uuid.New().String(), "unique_router_name", fifo, RouterDuplicate).
      TerminateLeft(
        NewTermination(uuid.New().String(), "unique_termination_name", !fifo, func(list []map[string]interface{}) error {

          // send a copy of the data somewhere

          return nil
        }),
      ).
      TerminateRight(
        NewTermination(uuid.New().String(), "unique_termination_name", !fifo, func(list []map[string]interface{}) error {

          // send a copy of the data somewhere else

          return nil
        }),
      ),

  // Build take an int buffer size and some Recorder functions to allow for state management or logging.
  // Having a Recorder requires the use of a Deep Copy operation which can be expensive depending on
  // the data being processed
  ).Build(bufferSize, func(vertexID, operation string, payload []*Packet) {})

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that one of the paths is not terminated
    panic(err)
  }
```

Incase of errors you can also route the errors down their own path with more complex flows having retry loops

```golang
  // fifo controls whether or not the data is processed in order of reciept
  fifo := false
  // bufferSize controls the go channel buffer size, set to 0 for unbuffered channels
  bufferSize := 0

  machineInstance := New(uuid.New().String(), "machine_name", fifo, func(c context.Context) chan []map[string]interface{} {
    return channel
  }).Then(
    NewVertex(uuid.New().String(), "unique_vertex_name", fifo, func(m map[string]interface{}) error {
      var err error

      // ...do some processing

      return err
    }).Route(
      NewRouter(uuid.New().String(), "unique_router_name", fifo, RouterError).
        TerminateLeft(
          NewTermination(uuid.New().String(), "unique_termination_name", !fifo, func(list []map[string]interface{}) error {

            // send successful data somewhere

            return nil
          }),
        ).
        TerminateRight(
          NewTermination(uuid.New().String(), "unique_termination_name", !fifo, func(list []map[string]interface{}) error {

            // send erroneous data somewhere else

            return nil
          }),
        ),
    ),

  // Build take an int buffer size and some Recorder functions to allow for state management or logging.
  // Having a Recorder requires the use of a Deep Copy operation which can be expensive depending on
  // the data being processed
  ).Build(bufferSize, func(vertexID, operation string, payload []*Packet) {})

  if err := machineInstance.Run(context.Background()); err != nil {
    // Run will return an error in the case that one of the paths is not terminated
    panic(err)
  }
```

Routing based on filtering the data is also possible with RouterRule

```golang  
  // fifo controls whether or not the data is processed in order of reciept
  fifo := false
  // bufferSize controls the go channel buffer size, set to 0 for unbuffered channels
  bufferSize := 0

  machineInstance := New(uuid.New().String(), "machine_name", fifo, func(c context.Context) chan []map[string]interface{} {
    return channel
  }).Then(
    NewVertex(uuid.New().String(), "unique_vertex_name", fifo, func(m map[string]interface{}) error {
      var err error

      // ...do some processing

      return err
    }).Route(
      NewRouter(uuid.New().String(), "unique_router_name", fifo, RouterRule(func(m map[string]interface{}) bool {
        return len(m) > 0
      }).Handler).
        TerminateLeft(
          NewTermination(uuid.New().String(), "unique_termination_name", !fifo, func(list []map[string]interface{}) error {

            // send correct data somewhere

            return nil
          }),
        ).
        TerminateRight(
          NewTermination(uuid.New().String(), "unique_termination_name", !fifo, func(list []map[string]interface{}) error {

            // send bad data somewhere else

            return nil
          }),
        ),
    ),

  // Build take an int buffer size and some Recorder functions to allow for state management or logging.
  // Having a Recorder requires the use of a Deep Copy operation which can be expensive depending on
  // the data being processed
  ).Build(bufferSize, func(vertexID, operation string, payload []*Packet) {})

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