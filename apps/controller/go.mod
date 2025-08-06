module github.com/DaiYuANg/warden/controller

go 1.24

require (
	github.com/DaiYuANg/warden/container v0.0.0-00010101000000-000000000000
	github.com/DaiYuANg/warden/dns v0.0.0-00010101000000-000000000000
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
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/hashicorp/mdns v1.0.6
	github.com/joho/godotenv v1.5.1
	github.com/knadh/koanf/parsers/json v1.0.0
	github.com/knadh/koanf/parsers/toml v0.1.0
	github.com/knadh/koanf/parsers/yaml v1.1.0
	github.com/knadh/koanf/providers/env v1.1.0
	github.com/knadh/koanf/providers/file v1.2.0
	github.com/knadh/koanf/providers/posflag v1.0.1
	github.com/knadh/koanf/providers/structs v1.0.0
	github.com/knadh/koanf/v2 v2.2.2
	github.com/panjf2000/ants/v2 v2.11.3
	github.com/samber/lo v1.51.0
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.7
	go.uber.org/fx v1.24.0
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.42.0
)

replace (
	github.com/DaiYuANg/warden/container => ./../../modules/container
	github.com/DaiYuANg/warden/dns => ./../../libs/dns
	github.com/DaiYuANg/warden/logger => ../../modules/logger
	github.com/DaiYuANg/warden/metadata => ./../../libs/metadata_db
	github.com/DaiYuANg/warden/pkg => ./../../libs/pkg
	github.com/DaiYuANg/warden/raft => ./../../libs/raft
	github.com/DaiYuANg/warden/schedule => ./../../modules/schedule
)

require (
	github.com/DaiYuANg/warden/logger v0.0.0-00010101000000-000000000000 // indirect
	github.com/DaiYuANg/warden/metadata v0.0.0-00010101000000-000000000000 // indirect
	github.com/DaiYuANg/warden/schedule v0.0.0-00010101000000-000000000000 // indirect
	github.com/MicahParks/keyfunc/v2 v2.1.0 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/benbjohnson/immutable v0.4.3 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/etcd v3.3.27+incompatible // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/pkg v0.0.0-20240122114842-bbd7aa9bf6fb // indirect
	github.com/dgraph-io/badger/v3 v3.2103.5 // indirect
	github.com/dgraph-io/ristretto v0.2.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/eko/gocache/lib/v4 v4.2.0 // indirect
	github.com/fasthttp/websocket v1.5.12 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.9 // indirect
	github.com/go-co-op/gocron/v2 v2.16.3 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.3 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-sockaddr v1.0.7 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/memberlist v0.5.3 // indirect
	github.com/hashicorp/raft v1.7.3 // indirect
	github.com/hashicorp/raft-wal v0.4.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/miekg/dns v1.1.68 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.23.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/samber/mo v1.14.0 // indirect
	github.com/savsgio/gotils v0.0.0-20250408102913-196191ec6287 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.64.0 // indirect
	go.etcd.io/bbolt v1.4.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/mock v0.5.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/exp v0.0.0-20250718183923-645b1fa84792 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/tools v0.35.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)
