module example.com/hostreach/subject

go 1.27

// THE SUBJECT REACHES THE RUNTIME because the check under test resolves a node
// constructor and the host accessor through go/types: both are declared by the
// runtime, and a fixture that stubbed them would prove the analyzer against the
// fixture author's idea of the runtime rather than against the runtime. A
// testdata module carries its own go.mod and is excluded from the parent module,
// so lang/analysis itself gains NO dependency from this.
require github.com/whitaker-io/machine/v4 v4.0.0

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
)

replace github.com/whitaker-io/machine/v4 => ../../../../..
