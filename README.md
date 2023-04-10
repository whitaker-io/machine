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



------

### **Installation**

Add the primary library to your project
```bash
  go get github.com/whitaker-io/machine/v2
```

------

The main function types are:

```golang
// Applicative is a function that is applied to payload and used for transformations
type Applicative[T any] func(d T) T

// Test is a function used in composition of And/Or operations and used to
// filter results down different branches with transformations
type Test[T any] func(d T) (T, error)

// Filter is a function that can be used to filter the payload.
type Filter[T any] func(d T) bool

// Transform is a function used by the Y Combinator to perform a recursion
// on the payload.
// Example:
// func(f Applicative[int]) Applicative[int] {
// 	 return func(x int) int {
// 		 if x < 1 {
// 			 return 1
// 		 } else {
// 			 return x * f(x-1)
// 		 }
// 	 }
// }
type Transform[T any] func(d Applicative[T]) Applicative[T]

```

These are used in the `Builder` type provided by the `Stream` type:


```golang
// Stream is a representation of a data stream and its associated logic.
//
// The Builder method is the entrypoint into creating the data processing flow.
// All branches of the Stream are required to end in an OutputTo call.
type Stream[T any] interface {
	Start(ctx context.Context, input chan T) error
	Builder() Builder[T]
}

// Builder is the interface provided for creating a data processing stream.
type Builder[T any] interface {
	Then(a Applicative[T]) Builder[T]
	Y(x Transform[T]) Builder[T]
	Or(x ...Test[T]) (Builder[T], Builder[T])
	And(x ...Test[T]) (Builder[T], Builder[T])
	Filter(f Filter[T]) (Builder[T], Builder[T])
	Duplicate() (Builder[T], Builder[T])
	Loop(x Filter[T]) (loop, out Builder[T])
	Drop()
	Distribute(Edge[T]) Builder[T]
	OutputTo(x chan T)
}
```

`Distribute` is a special method used for fan-out operations. It takes an instance of `Edge[T]` and can be used most typically to distribute work via a Pub/Sub. The `Edge[T]` interface is as follows:

```golang
// Edge is an interface that is used for transferring data between vertices
type Edge[T any] interface {
	ReceiveOn(ctx context.Context, channel chan T)
	Send(payload T)
}
```

The `Send` method is used for data leaving the associated vertex and the `ReceiveOn` method is used by the following vertex to receive data. The `context.Context` used is the same as the one used to start the `Stream`.

An example using google pubsub can be found in the `edge/pubsub` directory

------

You can also setup `Telemetry` and other options by passing in the `Option` type

```golang
// Option type for holding machine settings.
type Option[T any] struct {
	// FIFO controls the processing order of the payloads
	// If set to true the system will wait for one payload
	// to be processed before starting the next.
	FIFO bool `json:"fifo,omitempty"`
	// BufferSize sets the buffer size on the edge channels between the
	// vertices, this setting can be useful when processing large amounts
	// of data with FIFO turned on.
	BufferSize int `json:"buffer_size,omitempty"`
	// Telemetry provides the ability to enable and configure telemetry
	Telemetry Telemetry[T] `json:"telemetry,omitempty"`
	// PanicHandler is a function that is called when a panic occurs
	PanicHandler func(err error, payload T) `json:"-"`
	// DeepCopyBetweenVerticies controls whether DeepCopy is performed between verticies.
	// This is useful if the functions applied are holding copies of the payload for
	// longer than they process it. DeepCopy must be set
	DeepCopyBetweenVerticies bool `json:"deep_copy_between_vetricies,omitempty"`
	// DeepCopy is a function to preform a deep copy of the Payload
	DeepCopy func(T) T `json:"-"`
}

// Telemetry type for holding telemetry settings.
type Telemetry[T any] interface {
	IncrementPayloadCount(vertexName string)
	IncrementErrorCount(vertexName string)
	Duration(vertexName string, duration time.Duration)
	RecordPayload(vertexName string, payload T)
	RecordError(vertexName string, payload T, err error)
}
```

------

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
