package metrics

import "github.com/unionj-cloud/toolkit/gocache/lib/codec"

// MetricsInterface represents the metrics interface for all available providers
type MetricsInterface interface {
	RecordFromCodec(codec codec.CodecInterface)
}
