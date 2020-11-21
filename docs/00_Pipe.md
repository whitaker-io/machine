`Pipe` is the entrypoint into machine for a service that runs multiple `Stream`'s. Creating a new pipe is done with the `NewPipe` function who's signature is

```golang
func NewPipe(id string, logger Logger, store LogStore, config ...fiber.Config) *Pipe
```

This fields are as follows:

`id` is a unigue identifier for the pipe and used to identify the `Pipe` in uniquely in a cluster.

-----
`logger` is an interface used to send log infomation regarding failures or other status information to a log system

the interface is defined as follows:

```golang
type Logger interface {
  Error(...interface{})
  Info(...interface{})
}
```

-----
`store` is an interface used to communicate state in a distributed system. 

```golang
type LogStore interface {
  Join(id string, callback InjectionCallback, streamIDs ...string) error
  Write(logs ...*Log)
  Leave(id string) error
}
```

`Join` is called with the `Pipe` id, a callback for injecting dropped data back into the system, and the id's of all the `Stream`'s that the `Pipe` is running

-----

`Write` is called by a default recorder so that current state of the data flowing through a `Stream` can be communicated to peers

-----

`Leave` is called when the `Pipe` gracefully exits, letting the cluster know that it will no longer be available.

-----

The final argument is `config` a variadic of `fiber.Config` used for setting properties of the http server used to serve the healthcheck information and the HTTP based `Stream`'s

----

### Adding streams to the `Pipe`

The following methods can be used to add a new `Stream` to the `Pipe`

```golang
func (pipe *Pipe) Stream(stream Stream) Builder
```
The `Stream` method takes a stream that may or may not have been built yet and returns the `Builder`, you are free to ignore the `Builder` if the `Stream has already been setup.

-----

```golang
func (pipe *Pipe) StreamHTTP(id string, opts ...*Option) Builder
```
The `StreamHTTP` method takes an id and a variadic of `*machine.Option`s and returns the `Builder`. This method adds an HTTP route to the `Pipe`'s underlying fiber instance under the path `/stream/:id`

-----
```golang
func (pipe *Pipe) StreamSubscription(id string, sub Subscription, interval time.Duration, opts ...*Option) Builder
```
The `StreamSubscription` method takes an id, an implementation of Subscription, a time interval between the calls to `Read`, and a variadic of `*machine.Option`s and returns the `Builder`.