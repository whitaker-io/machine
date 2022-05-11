![Go](https://github.com/whitaker-io/machine/workflows/Go/badge.svg?branch=master)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/whitaker-io/machine)](https://pkg.go.dev/github.com/whitaker-io/machine)
[![GoDoc](https://godoc.org/github.com/whitaker-io/machine?status.svg)](https://godoc.org/github.com/whitaker-io/machine)
[![Go Report Card](https://goreportcard.com/badge/github.com/whitaker-io/machine)](https://goreportcard.com/report/github.com/whitaker-io/machine)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=whitaker-io/machine&amp;utm_campaign=Badge_Grade)
[![Codacy Badge](https://app.codacy.com/project/badge/Coverage/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&utm_medium=referral&utm_content=whitaker-io/machine&utm_campaign=Badge_Coverage)
[![Version Badge](https://img.shields.io/github/v/tag/whitaker-io/machine)](https://img.shields.io/github/v/tag/whitaker-io/machine)

<p align="center">
    <img alt="Machine" height="125" src="https://raw.githubusercontent.com/whitaker-io/machine/master/docs/static/Black-No-BG.png">
</p>

`Machine` is a library for creating data workflows. These workflows can be either very concise or quite complex, even allowing for cycles for flows that need retry or self healing mechanisms.

Implementations of the `EdgeProvider` and `Edge` interfaces can be used for fan-out so that large volumes of data can be process by multiple nodes.

```golang
type EdgeProvider[T Identifiable] interface {
	New(name string, option *Option[T]) Edge[T]
}

type Edge[T Identifiable] interface {
	OutputTo(ctx context.Context, channel chan []T)
	Input(payload ...T)
}
```

Examples of using both AWS SQS and Google Pub/Sub coming soon!

------

The main function types are:

```golang
// Applicative is a function that is applied on an individual
// basis for each Packet in the payload. The resulting data replaces
// the old data
type Applicative[T Identifiable] func(d T) T

// Combiner is a function used to combine a payload into a single Packet.
type Combiner[T Identifiable] func(payload []T) T

// Filter is a function that can be used to filter the payload.
type Filter[T Identifiable] func(d T) FilterResult

// Comparator is a function to compare 2 items
type Comparator[T Identifiable] func(a T, b T) int

// Window is a function to work on a window of data
type Window[T Identifiable] func(payload []T) []T

// Remover func that is used to remove Data based on a true result
type Remover[T Identifiable] func(index int, d T) bool
```

These all implement the `Component` which can be use to provide a `Vertex`. The `Vertex` is the main building block of a 
`Stream` under the covers and can be used individually for testing of very simple single operations.


```golang
// Component is an interface for providing a Vertex that can be used to run individual components on the payload.
type Component[T Identifiable] interface {
	Component(e Edge[T]) Vertex[T]
}
```

------

### **Installation**

Add the primary library to your project
```bash
  go get -u github.com/whitaker-io/machine
```
## [Example](#example)


Basic `receive` -> `process` -> `send` Flow

```golang
package main

import (
  "context"
  "fmt"
  "os"
  "os/signal"
  "time"

  "github.com/whitaker-io/machine"
)

type kv struct {
  name  string
  value int
}

func (i *kv) ID() string {
  return i.name
}

func deepcopyKV(k *kv) *kv { return &kv{name: k.name, value: k.value} }

func main() {
  ctx, cancel := context.WithCancel(context.Background())

  stream := machine.NewWithChannels(
    "test_stream",
    &Option[*kv]{
      DeepCopy:   deepcopyKV,
      FIFO:       false,
      BufferSize: 0,
    },
  )

  input := make(chan []*kv)
  out := make(chan []*kv)


  go func() {
    input <- someData //get some data from somewhere
  }()

  // this is a very simple example try experimenting 
  // with more complex flows including loops and filters
  stream.Builder().
    Map(
      func(m *kv) *kv {
        // do some processing
        return m
      },
    ).OutputTo(out)

  if err := streamm.StartWith(ctx, input); err != nil {
    fmt.Println(err)
  }


  go func() {
  Loop:
    for {
      select {
      case <-ctx.Done():
				break Loop
      case data := <-out:
      //handle the processed data
			}
		}
	}()

  // run until SIGTERM
  c := make(chan os.Signal, 1)
  signal.Notify(c, os.Interrupt)

  <-c
  cancel()

  // give some time for a graceful shutdown
  <-time.After(time.Second * 2)
}
```

***
## 🤝 Contributing

Contributions, issues and feature requests are welcome.<br />
Feel free to check [issues page](https://github.com/whitaker-io/machine/issues) if you want to contribute.<br />
[Check the contributing guide](./CONTRIBUTING.md).<br />

## Author

👤 **Jonathan Whitaker**

- Twitter: [@io_whitaker](https://twitter.com/io_whitaker)
- Github: [@jonathan-whitaker](https://github.com/jonathan-whitaker)

## Show your support

Please ⭐️ this repository if this project helped you!

***
## [License](#license)

Machine is provided under the [MIT License](https://github.com/whitaker-io/machine/blob/master/LICENSE).

```text
The MIT License (MIT)

Copyright (c) 2020 Jonathan Whitaker
```
