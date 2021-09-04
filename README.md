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

It supports 
[opentelemetry](https://github.com/open-telemetry/opentelemetry-go) spans and metrics out of the box 

It also supports building dynamic pipelines using 
- native go plugins 
- [hashicorp](https://github.com/hashicorp/go-plugin)
- [yaegi](https://github.com/traefik/yaegi)
- [tengo](https://github.com/d5/tengo)

[Components](https://github.com/whitaker-io/components) is a repository of different vertex and plugin implementations 

------

### **Installation**

Add the primary library to your project
```bash
  go get -u github.com/whitaker-io/machine
```


------

[Data](https://github.com/whitaker-io/data) is a library for getting and setting values in a `map`[`string`]`interface`{} 

------

## [Documentation](#docs)

<img alt="Gopher" align="right" height="250" src="https://raw.githubusercontent.com/whitaker-io/machine/master/docs/static/Gopher.png">

[Docs](./docs)
  * [Stream](./docs/01_Stream.md)
  * [Map](./docs/02_Map.md)
  * [Fork](./docs/03_Fork.md)
  * [Fold](./docs/04_Fold.md)
  * [Loop](./docs/07_Loop.md)
  * [Sort](./docs/09_Sort.md)
  * [Remove](./docs/10_Remove.md)
  * [Publish](./docs/06_Publisher.md)
  * [Plugins](./docs/08_Plugins.md)

***
## [Example](#example)


Basic `receive` -> `process` -> `send` Flow

```golang
  stream := NewStream("unique_id1", 
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

  stream.Builder().Map("unique_id2", 
      func(m Data) error {
        var err error

        // ...do some processing

        return err
      },
    ).Publish("publish_left_id", publishFN(func(d []data.Data) error {
      // send the data somewhere

      return nil
    }),
  )

  if err := stream.Run(context.Background()); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated (i.e. missing a Publish)
    panic(err)
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
