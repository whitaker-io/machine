![Go](https://github.com/whitaker-io/machine/workflows/Go/badge.svg?branch=master)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/whitaker-io/machine)](https://pkg.go.dev/github.com/whitaker-io/machine)
[![GoDoc](https://godoc.org/github.com/whitaker-io/machine?status.svg)](https://godoc.org/github.com/whitaker-io/machine)
[![Go Report Card](https://goreportcard.com/badge/github.com/whitaker-io/machine)](https://goreportcard.com/report/github.com/whitaker-io/machine)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=whitaker-io/machine&amp;utm_campaign=Badge_Grade)
[![Codacy Badge](https://app.codacy.com/project/badge/Coverage/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&utm_medium=referral&utm_content=whitaker-io/machine&utm_campaign=Badge_Coverage)
[![Version Badge](https://img.shields.io/github/v/tag/whitaker-io/machine)](https://img.shields.io/github/v/tag/whitaker-io/machine)
[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/whitaker-io/machine)

<p align="center">
    <img alt="Machine" height="125" src="https://raw.githubusercontent.com/whitaker-io/machine/master/docs/static/Black-No-BG.png">
</p>

`Machine` is a library for creating data workflows. These workflows can be either very concise or quite complex, even allowing for cycles for flows that need retry or self healing mechanisms. It supports [opentelemetry](https://github.com/open-telemetry/opentelemetry-go) spans and metrics out of the box and supports building dynamic pipelines using native go plugins and [hashicorp](https://github.com/hashicorp/go-plugin) or [yaegi](https://github.com/traefik/yaegi) based plugins by using the providers [here](https://github.com/whitaker-io/components).

### **Installation**

Add the primary library to your project
```bash
  go get -u github.com/whitaker-io/machine
```

------

[Components](https://github.com/whitaker-io/components) is a repository of different vertex implementations 

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
  * [Publish](./docs/06_Publisher.md)
  * [Plugins](./docs/08_Plugins.md)

***
## [Example](#example)


Redis Subscription with basic `receive` -> `process` -> `send` Stream

```golang
  // logStore allows for running a cluster and handles communication
  var logStore machine.LogStore

  // pool is a redigo Pool for a redis cluster to read the stream from
  // see also the Google Pub/Sub, Kafka, and SQS implementations
  var pool *redigo.Pool

  // publisher is a machine.Publisher used for sending data outside of the Stream
  var publisher machine.Publisher

  redisStream := redis.New(pool, logger)
  
  // NewPipe creates a pipe in which you can run multiple streams
  // the id is the instance identifier for the cluster
  p := NewPipe(uuid.New().String(), nil, logStore, fiber.Config{
    ReadTimeout: time.Second,
    WriteTimeout: time.Second,
    BodyLimit: 4 * 1024 * 1024,
    DisableKeepalive: true,
  })

  // StreamSubscription takes an instance of machine.Subscription
  // and a time interval in which to read
  // the id here needs to be the same for all the nodes for the clustering to work
  builder := p.StreamSubscription("unique_stream_id", redisStream, 5*time.Millisecond,
    &Option{FIFO: boolP(false)},
    &Option{Injectable: boolP(true)},
    &Option{Metrics: boolP(true)},
    &Option{Span: boolP(false)},
    &Option{BufferSize: intP(0)},
  ).Builder()

  builder.Map("unique_id2", 
      func(m Data) error {
        var err error

        // ...do some processing

        return err
      },
    ).
    Publish("unique_id3", publisher)

  // Run requires a context, the port to run the fiber.App,
  // and the timeout for graceful shutdown
  if err := p.Run(context.Background(), ":5000", 10 * time.Second); err != nil {
    // Run will return an error in the case that 
    // one of the paths is not terminated (i.e. missing a Transmit)
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
