module github.com/DaiYuANg/warden/controller

go 1.24

require (
	github.com/DaiYuANg/warden/logger v0.0.0-00010101000000-000000000000
	github.com/DaiYuANg/warden/pkg v0.0.0-00010101000000-000000000000
	github.com/DaiYuANg/warden/raft v0.0.0-00010101000000-000000000000
	github.com/adrg/xdg v0.5.3
	github.com/ansrivas/fiberprometheus/v2 v2.13.0
	github.com/danielgtaylor/huma/v2 v2.34.1
	github.com/go-playground/validator/v10 v10.27.0
	github.com/gofiber/contrib/fiberzap/v2 v2.1.6
	github.com/gofiber/contrib/jwt v1.1.2
	github.com/gofiber/contrib/websocket v1.3.4
	github.com/gofiber/fiber/v2 v2.52.9
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/hashicorp/mdns v1.0.6
	github.com/joho/godotenv v1.5.1
	github.com/knadh/koanf/v2 v2.2.2
	github.com/miekg/dns v1.1.67
	github.com/samber/lo v1.51.0
	github.com/spf13/cobra v1.9.1
	go.uber.org/fx v1.24.0
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.42.0
)

replace (
	github.com/DaiYuANg/warden/logger => ./../modules/logger
	github.com/DaiYuANg/warden/pkg => ./../modules/pkg
	github.com/DaiYuANg/warden/raft => ./../modules/raft
)

require (
	github.com/MicahParks/keyfunc/v2 v2.1.0 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgraph-io/badger/v3 v3.2103.5 // indirect
	github.com/dgraph-io/ristretto v0.2.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fasthttp/websocket v1.5.8 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.9 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/flatbuffers v1.12.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-hclog v1.6.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/raft v1.7.3 // indirect
	github.com/hashicorp/raft-boltdb v0.0.0-20250701115049-6cdf087e85ed // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/savsgio/gotils v0.0.0-20240303185622-093b76447511 // indirect
	github.com/spf13/pflag v1.0.7 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.64.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/tools v0.35.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
