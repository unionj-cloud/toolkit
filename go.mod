module github.com/unionj-cloud/toolkit

go 1.22.2

require (
	github.com/ascarter/requestid v0.0.0-20170313220838-5b76ab3d4aee
	github.com/go-git/go-billy/v5 v5.4.1
	github.com/go-git/go-git/v5 v5.6.1
	github.com/go-resty/resty/v2 v2.7.0
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/hyperjumptech/jiffy v1.0.0
	github.com/iancoleman/strcase v0.3.0
	github.com/jeremywohl/flatten v1.0.1
	github.com/jmoiron/sqlx v1.3.5
	github.com/joho/godotenv v1.5.1
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.19.0
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.10.0
	github.com/uber/jaeger-client-go v2.30.0+incompatible
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	golang.org/x/tools v0.26.0
)

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/XiaoMi/pegasus-go-client v0.0.0-20220519103347-ba0e68465cd5
	github.com/allegro/bigcache/v3 v3.1.0
	github.com/auxten/postgresql-parser v1.0.1
	github.com/bradfitz/gomemcache v0.0.0-20230905024940-24af94b03874
	github.com/bytedance/sonic v1.13.3
	github.com/coocood/freecache v1.2.4
	github.com/deckarep/golang-set/v2 v2.6.0
	github.com/dgraph-io/ristretto v0.1.1
	github.com/go-playground/assert/v2 v2.2.0
	github.com/gobuffalo/flect v1.0.3
	github.com/goccy/go-reflect v1.2.0
	github.com/hashicorp/go-sockaddr v1.0.2
	github.com/hazelcast/hazelcast-go-client v1.4.1
	github.com/mholt/archiver/v3 v3.5.1
	github.com/morkid/gocache v1.0.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/proullon/ramsql v0.1.4
	github.com/redis/rueidis v1.0.38
	github.com/redis/rueidis/mock v1.0.38
	github.com/redis/rueidis/rueidiscompat v1.0.38
	github.com/samber/lo v1.39.0
	github.com/wubin1989/gorm v0.0.5
	github.com/wubin1989/gorm-dameng v0.5.1
	github.com/wubin1989/mysql v0.0.2
	github.com/wubin1989/sqlite v0.0.3
	github.com/xwb1989/sqlparser v0.0.0-20180606152119-120387863bf2
	go.uber.org/mock v0.4.0
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.1.2 // indirect
	github.com/apache/thrift v0.16.0 // indirect
	github.com/bytedance/sonic/loader v0.2.4 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/certifi/gocertifi v0.0.0-20200922220541-2c3bb06c6054 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/cockroachdb/apd v1.1.1-0.20181017181144-bced77f817b4 // indirect
	github.com/cockroachdb/errors v1.8.2 // indirect
	github.com/cockroachdb/logtags v0.0.0-20190617123548-eb05cc24525f // indirect
	github.com/cockroachdb/redact v1.0.8 // indirect
	github.com/cockroachdb/sentry-go v0.6.1-cockroachdb.2 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/getsentry/raven-go v0.2.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/golang/glog v1.2.0 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/pegasus-kv/thrift v0.13.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/shirou/gopsutil/v3 v3.23.12 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	golang.org/x/arch v0.1.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/tomb.v2 v2.0.0-20161208151619-d5d1b5820637 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.26.2 // indirect
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20230217124315-7d5c6f04bbb8 // indirect
	github.com/acomagu/bufpipe v1.0.4 // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/antlr/antlr4 v0.0.0-20200124162019-2d7f727a00b7 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudflare/circl v1.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dsnet/compress v0.0.2-0.20210315054119-f66993602bf5 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.14 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20181017120253-0766667cb4d1 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/invopop/yaml v0.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.17.8
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.31.1
	github.com/perimeterx/marshmallow v1.1.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.48.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/redis/go-redis/v9 v9.1.0
	github.com/skeema/knownhosts v1.1.0 // indirect
	github.com/smartystreets/assertions v1.2.0
	github.com/spf13/cast v1.3.0
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/vmihailenco/go-tinylfu v0.2.2
	github.com/vmihailenco/msgpack/v5 v5.3.5
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/mod v0.21.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sync v0.10.0
	golang.org/x/sys v0.26.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

require (
	github.com/armon/go-metrics v0.4.1
	github.com/getkin/kin-openapi v0.115.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-playground/form/v4 v4.2.0
	github.com/golang/mock v1.6.0
	github.com/google/btree v1.1.2
	github.com/google/uuid v1.6.0
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-msgpack v1.1.5
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jackc/pgx/v5 v5.5.5 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/lithammer/shortuuid/v4 v4.0.0
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/miekg/dns v1.1.54
	github.com/rs/zerolog v1.28.0
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/shopspring/decimal v1.4.0
	github.com/smartystreets/goconvey v1.7.2
	github.com/wubin1989/postgres v0.0.2
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto v0.0.0-20230920204549-e6e6cdab5c13 // indirect
	google.golang.org/grpc v1.64.1 // indirect
)
