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

The two function types are:

```golang
// Monad is a function that is applied to payload and used for transformations
type Monad[T any] func(d T) T

// Filter is a function that can be used to filter the payload.
type Filter[T any] func(d T) bool

```

These are used in the `Machine` for functional operations

```golang
// New is a function for creating a new Machine.
//
// name string
// input chan T
// option *Option[T]
//
// Call the startFn returned by New to start the Machine once built.
func New[T any](name string, input chan T, options ...Option) (startFn func(context.Context), x Machine[T])

// Transform is a function for converting the type of the Machine. Cannot be used inside a loop
// until I figure out how to do it without some kind of run time error or overly complex
// tracking method that isn't type safe. I really wish method level generics were a thing.
func Transform[T, U any](m Machine[T], fn func(d T) U) (Machine[U], error)

// Machine is the interface provided for creating a data processing stream.
type Machine[T any] interface {
	// Name returns the name of the Machine path. Useful for debugging or reasoning about the path.
	Name() string
	// Then apply a mutation to each individual element of the payload.
	Then(a Monad[T]) Machine[T]
	// Recurse applies a recursive function to the payload through a Y Combinator.
	// f is a function used by the Y Combinator to perform a recursion
	// on the payload.
	// Example:
	//
	//	func(f Monad[int]) Monad[int] {
	//		 return func(x int) int {
	//			 if x <= 0 {
	//				 return 1
	//			 } else {
	//				 return x * f(x-1)
	//			 }
	//		 }
	//	}
	Recurse(x Monad[Monad[T]]) Machine[T]
	// Memoize applies a recursive function to the payload through a Y Combinator
	// and memoizes the results based on the index func.
	// f is a function used by the Y Combinator to perform a recursion
	// on the payload.
	// Example:
	//
	//	func(f Monad[int]) Monad[int] {
	//		 return func(x int) int {
	//			 if x <= 0 {
	//				 return 1
	//			 } else {
	//				 return x * f(x-1)
	//			 }
	//		 }
	//	}
	Memoize(x Monad[Monad[T]], index func(T) string) Machine[T]
	// Or runs all of the functions until one succeeds or sends the payload to the right branch
	Or(x ...Filter[T]) (Machine[T], Machine[T])
	// And runs all of the functions and if one doesnt succeed sends the payload to the right branch
	And(x ...Filter[T]) (Machine[T], Machine[T])
	// Filter splits the data into multiple stream branches
	If(f Filter[T]) (Machine[T], Machine[T])
	// Select applies a series of Filters to the payload and returns a list of Builders
	// the last one being for any unmatched payloads.
	Select(fns ...Filter[T]) []Machine[T]
	// Tee duplicates the data into multiple stream branches.
	Tee(func(T) (a, b T)) (Machine[T], Machine[T])
	// While creates a loop in the stream based on the filter
	While(x Filter[T]) (loop, out Machine[T])
	// Drop terminates the data from further processing without passing it on
	Drop()
	// Distribute is a function used for fanout
	Distribute(Edge[T]) Machine[T]
	// Output provided channel
	Output() chan T
}
```

`Distribute` is a special method used for fan-out operations. It takes an instance of `Edge[T]` and can be used most typically to distribute work via a Pub/Sub or it can be used in a commandline utility to handle user input or a similiar blocking process. 


The `Edge[T]` interface is as follows:

```golang
// Edge is an interface that is used for transferring data between vertices
type Edge[T any] interface {
	Output() chan T
	Send(payload T)
}
```

The `Send` method is used for data leaving the associated vertex and the `Output` method is used by the following vertex to receive data from the channel.

------

Confirguration is done using the `Option` helper

```golang
// Option is used to configure the machine
type Option interface {
	apply(*config)
}

// OptionFIF0 controls the processing order of the payloads
// If set to true the system will wait for one payload
// to be processed before starting the next.
var OptionFIF0 Option

// OptionBufferSize sets the buffer size on the edge channels between the
// vertices, this setting can be useful when processing large amounts
// of data with FIFO turned on.
func OptionBufferSize(size int) Option

// OptionDebug enables debug logging for the machine
var OptionDebug Option
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
