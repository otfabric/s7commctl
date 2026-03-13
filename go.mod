module github.com/otfabric/s7commctl

go 1.25.0

require (
	github.com/spf13/cobra v1.10.2
	otfabric/s7comm v0.1.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

replace otfabric/s7comm => github.com/otfabric/s7comm v0.1.0
