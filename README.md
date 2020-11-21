![Go](https://github.com/whitaker-io/machine/workflows/Go/badge.svg?branch=master)
[![GoDoc](https://godoc.org/github.com/whitaker-io/machine?status.svg)](https://godoc.org/github.com/whitaker-io/machine)
[![Go Report Card](https://goreportcard.com/badge/github.com/whitaker-io/machine)](https://goreportcard.com/report/github.com/whitaker-io/machine)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=whitaker-io/machine&amp;utm_campaign=Badge_Grade)
[![Codacy Badge](https://app.codacy.com/project/badge/Coverage/aa8efa7beb3f4e66a5dc0247e25557b5)](https://www.codacy.com?utm_source=github.com&utm_medium=referral&utm_content=whitaker-io/machine&utm_campaign=Badge_Coverage)
[![Gitter chat](https://badges.gitter.im/whitaker-io/machine.png)](https://gitter.im/whitaker-io/machine)

`Machine` is a library for creating data workflows. These workflows can be either very concise or quite complex, even allowing for cycles for flows that need retry or self healing mechanisms.

### **Installation**

Add the primary library to your project
```bash
  go get -u github.com/whitaker-io/machine
```
------
Download the command to generate a boilerplate project (**WIP/alpha state**)
```bash
  go get -u github.com/whitaker-io/machine/cmd
  cd $GOSRC/github.com/whitaker-io/machine/cmd
  go build -a -o pkg/machine -ldflags "-s -w"
  sudo ln -s pkg/machine /usr/local/bin/machine
```
------
Add the different Subscription Implementations
```bash
  go get -u github.com/whitaker-io/machine/subscriptions/kafka
```
```bash
  go get -u github.com/whitaker-io/machine/subscriptions/redis
```
```bash
  go get -u github.com/whitaker-io/machine/subscriptions/pubsub
```
```bash
  go get -u github.com/whitaker-io/machine/subscriptions/sqs
```

***
## [Documentation](#docs)

[Docs](./docs)
  * [Pipe](./docs/00_Pipe.md)
  * [Stream](./docs/01_Stream.md)
  * [Map](./docs/02_Map.md)
  * [Fork](./docs/03_Fork.md)
  * [Fold](./docs/04_Fold.md)
  * [Link](./docs/05_Link.md)
  * [Transmit](./docs/06_Transmit.md)

## 🤝 Contributing

Contributions, issues and feature requests are welcome.<br />
Feel free to check [issues page](https://github.com/whitaker-io/machine/issues) if you want to contribute.<br />
[Check the contributing guide](./CONTRIBUTING.md).<br />

## Author

👤 **Jonathan Whitaker**

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