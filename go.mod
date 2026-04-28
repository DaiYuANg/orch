module github.com/daiyuang/orch

// The `go` line is the minimum language version for this module; `go mod tidy` raises it to match
// transitive requirements (currently 1.26.x). Toolchain selection must be >= this `go` line — you
// cannot pin toolchain below `go` to “fix” a dependency; resolve outdated deps via version bumps or
// `replace`. Use GOTOOLCHAIN=local only to forbid auto-download of another toolchain (offline CI).
go 1.26.2

require (
	github.com/adrg/xdg v0.5.3
	github.com/ansrivas/fiberprometheus/v2 v2.17.0
	github.com/arcgolabs/authx v0.1.0
	github.com/arcgolabs/authx/http/fiber v0.1.0
	github.com/arcgolabs/authx/jwt v0.1.0
	github.com/arcgolabs/clientx v0.1.0
	github.com/arcgolabs/collectionx/list v0.2.0
	github.com/arcgolabs/collectionx/mapping v0.2.0
	github.com/arcgolabs/collectionx/set v0.2.0
	github.com/arcgolabs/configx v0.3.0
	github.com/arcgolabs/dix v0.6.0
	github.com/arcgolabs/dnsx/dnsserver v0.0.0-20260424133130-d97fd7154630
	github.com/arcgolabs/httpx v0.1.1
	github.com/arcgolabs/httpx/adapter/fiber v0.1.1
	github.com/arcgolabs/logx v0.1.0
	github.com/arcgolabs/observabilityx v0.2.0
	github.com/arcgolabs/storx/badgerx v0.1.0
	github.com/arcgolabs/storx/bboltx v0.1.0
	github.com/arcgolabs/storx/codec v0.1.0
	github.com/arcgolabs/storx/keycodec v0.1.0
	github.com/caddyserver/caddy/v2 v2.11.2
	github.com/compose-spec/compose-go/v2 v2.9.0
	github.com/containerd/containerd v1.7.31
	github.com/containerd/errdefs v1.0.0
	github.com/danielgtaylor/huma/v2 v2.37.3
	github.com/dgraph-io/badger/v4 v4.9.1
	github.com/docker/docker v28.5.2+incompatible
	github.com/go-co-op/gocron/v2 v2.21.1
	github.com/gofiber/fiber/v2 v2.52.13
	github.com/hashicorp/go-hclog v1.6.3
	github.com/hashicorp/raft v1.7.3
	github.com/miekg/dns v1.1.72
	github.com/prometheus/client_golang v1.23.2
	github.com/samber/lo v1.53.0
	github.com/samber/oops v1.21.0
	github.com/shirou/gopsutil/v4 v4.26.3
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/vishvananda/netns v0.0.5
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cel.dev/expr v0.25.1 // indirect
	cloud.google.com/go/auth v0.20.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cyphar.com/go-pathrs v0.2.4 // indirect
	dario.cat/mergo v1.0.2 // indirect
	filippo.io/bigmod v0.1.0 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6 // indirect
	github.com/AdamKorcz/go-118-fuzz-build v0.0.0-20250520111509-a70c2aa677fa // indirect
	github.com/AndreasBriese/bbloom v0.0.0-20190825152654-46b345b51c96 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/DeRuina/timberjack v1.4.2 // indirect
	github.com/DmitriyVTitov/size v1.5.0 // indirect
	github.com/KimMachineGun/automemlimit v0.7.5 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/Masterminds/sprig/v3 v3.3.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.14.1 // indirect
	github.com/alecthomas/chroma/v2 v2.23.1 // indirect
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/arcgolabs/collectionx v0.2.0 // indirect
	github.com/arcgolabs/collectionx/internal v0.2.0 // indirect
	github.com/arcgolabs/collectionx/interval v0.2.0 // indirect
	github.com/arcgolabs/collectionx/prefix v0.2.0 // indirect
	github.com/arcgolabs/collectionx/tree v0.2.0 // indirect
	github.com/arcgolabs/dnsx/dnsclient v0.0.0-20260424133130-d97fd7154630 // indirect
	github.com/arcgolabs/httpx/adapter/echo v0.1.1 // indirect
	github.com/arcgolabs/httpx/adapter/gin v0.1.1 // indirect
	github.com/arcgolabs/httpx/adapter/std v0.1.1 // indirect
	github.com/arcgolabs/pkg/option v0.0.3 // indirect
	github.com/arcgolabs/storx v0.1.0 // indirect
	github.com/arcgolabs/storx/observer v0.1.0 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aryann/difflib v0.0.0-20210328193216-ff5ff6dc229b // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/caddyserver/certmagic v0.25.3 // indirect
	github.com/caddyserver/zerossl v0.1.5 // indirect
	github.com/ccoveille/go-safecast/v2 v2.0.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/containerd/cgroups/v3 v3.1.3 // indirect
	github.com/containerd/containerd/api v1.10.0 // indirect
	github.com/containerd/continuity v0.4.5 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.4 // indirect
	github.com/containerd/ttrpc v1.2.8 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/coreos/go-oidc/v3 v3.18.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/dgraph-io/badger v1.6.2 // indirect
	github.com/dgraph-io/badger/v2 v2.2007.4 // indirect
	github.com/dgraph-io/ristretto v0.2.0 // indirect
	github.com/dgraph-io/ristretto/v2 v2.4.0 // indirect
	github.com/dgryski/go-farm v0.0.0-20240924180020-3414d57e47da // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/docker/go-connections v0.7.0 // indirect
	github.com/docker/go-events v0.0.0-20250808211157-605354379745 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-chi/chi/v5 v5.2.5 // indirect
	github.com/go-jose/go-jose/v3 v3.0.5 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.2 // indirect
	github.com/go-sql-driver/mysql v1.9.3 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/cel-go v0.28.0 // indirect
	github.com/google/certificate-transparency-go v1.3.3 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/go-tspi v0.3.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.15 // indirect
	github.com/googleapis/gax-go/v2 v2.22.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.5 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.9.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/parsers/json v1.0.0 // indirect
	github.com/knadh/koanf/parsers/toml/v2 v2.2.0 // indirect
	github.com/knadh/koanf/parsers/yaml v1.1.0 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/providers/env/v2 v2.0.0 // indirect
	github.com/knadh/koanf/providers/file v1.2.1 // indirect
	github.com/knadh/koanf/v2 v2.3.4 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/libdns/libdns v1.1.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20260330125221-c963978e514e // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/mattn/go-shellwords v1.0.12 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mholt/acmez/v3 v3.1.6 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/atomicwriter v0.1.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/morikuni/aec v1.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/opencontainers/selinux v1.13.1 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/pelletier/go-toml/v2 v2.3.0 // indirect
	github.com/pires/go-proxyproto v0.12.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/samber/do/v2 v2.0.0 // indirect
	github.com/samber/go-singleflightx v0.3.2 // indirect
	github.com/samber/go-type-to-string v1.8.0 // indirect
	github.com/samber/hot v0.13.0 // indirect
	github.com/samber/mo v1.16.0 // indirect
	github.com/samber/oops/loggers/zerolog v0.0.0-20260412154111-1460827f264f // indirect
	github.com/samber/slog-common v0.22.0 // indirect
	github.com/samber/slog-zerolog/v2 v2.9.2 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/slackhq/nebula v1.10.3 // indirect
	github.com/smallstep/certificates v0.30.2 // indirect
	github.com/smallstep/cli-utils v0.12.2 // indirect
	github.com/smallstep/go-attestation v0.4.9 // indirect
	github.com/smallstep/linkedca v0.25.0 // indirect
	github.com/smallstep/nosql v0.8.0 // indirect
	github.com/smallstep/pkcs7 v0.2.1 // indirect
	github.com/smallstep/scep v0.0.0-20260331191114-261f960a40d1 // indirect
	github.com/smallstep/truststore v0.13.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/tailscale/go-winio v0.0.0-20231025203758-c4f33415bf55 // indirect
	github.com/tailscale/tscert v0.0.0-20251216020129-aea342f6d747 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/urfave/cli v1.22.17 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.70.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	github.com/yuin/goldmark v1.8.2 // indirect
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	go.etcd.io/bbolt v1.4.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/bridges/prometheus v0.68.0 // indirect
	go.opentelemetry.io/contrib/exporters/autoexport v0.68.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.68.0 // indirect
	go.opentelemetry.io/contrib/propagators/autoprop v0.68.0 // indirect
	go.opentelemetry.io/contrib/propagators/aws v1.43.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.43.0 // indirect
	go.opentelemetry.io/contrib/propagators/jaeger v1.43.0 // indirect
	go.opentelemetry.io/contrib/propagators/ot v1.43.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.19.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.19.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.65.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.19.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.43.0 // indirect
	go.opentelemetry.io/otel/log v0.19.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.19.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.step.sm/crypto v0.77.7 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20260423152011-b9e53593a607 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/term v0.42.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.44.0 // indirect
	google.golang.org/api v0.276.0 // indirect
	google.golang.org/genproto v0.0.0-20260427160629-7cedc36a6bc4 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260427160629-7cedc36a6bc4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260427160629-7cedc36a6bc4 // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.6.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gotest.tools/v3 v3.5.2 // indirect
	howett.net/plist v1.0.1 // indirect
	resty.dev/v3 v3.0.0-beta.6 // indirect
)
