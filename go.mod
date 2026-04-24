module github.com/Cloud-SPE/openai-worker-node

go 1.25

// Local sibling checkout until the library tags a release. Workers
// deploying from tags drop the replace and pin a version.
replace github.com/Cloud-SPE/livepeer-payment-library => ../livepeer-payment-library

require (
	github.com/Cloud-SPE/livepeer-payment-library v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.80.0
)

require (
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260120221211-b8f7ae30c516 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
