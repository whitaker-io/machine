# Plugins

## Overview
Plugins provide a way for vertex functions to be declared and run dynamically. This provides the Developer with flexibility in the systems that they run. This flexibility though useful comes with the drawback of a minor performance hit, so the Developer must take this into consideration when processing large amounts of data.

### Types

[PluginDefinition](https://pkg.go.dev/github.com/whitaker-io/machine#PluginDefinition) - is a type for holding configuration used to load a Vertex Provider from a PluginProvider.

----

[PluginProvider](https://pkg.go.dev/github.com/whitaker-io/machine#PluginProvider) - is an interface to provide the Vertex Provider functions. 

This interface has a single method 
```golang
func Load(*machine.PluginDefinition) (interface{}, error)
```

This method takes a *`machine`.`PluginDefinition` and uses that information to create a Vertex Type. The possible Vertex Types are as follows:

```golang
  machine.Subscription
  machine.Retriever
  machine.Applicative
  machine.Fold
  machine.Fork
  machine.Publisher
```

----

[StreamSerialization](https://pkg.go.dev/github.com/whitaker-io/machine#StreamSerialization) - is a type for holding configuration used to define a `Steam` that will be run in the `Pipe`. It is used with the subtype:

[VertexSerialization](https://pkg.go.dev/github.com/whitaker-io/machine#VertexSerialization) - type for holding configuration of individual Vertices in a `Stream`

## Examples
