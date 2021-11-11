module github.com/debugtalk/boomer

go 1.16

require (
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/bugVanisher/grequester v0.0.0-20201029025943-70cb6f2c7295
	github.com/coreos/bbolt v1.3.6 // indirect
	github.com/coreos/etcd v3.3.27+incompatible // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.0
	github.com/google/btree v1.0.1 // indirect
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/olekukonko/tablewriter v0.0.5
	github.com/prometheus/client_golang v1.11.0 // indirect
	github.com/shirou/gopsutil v3.21.10+incompatible
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20201229170055-e5319fda7802 // indirect
	github.com/ugorji/go/codec v1.2.6
	github.com/valyala/fasthttp v1.31.0
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	github.com/yuin/gopher-lua v0.0.0-20210529063254-f4c35e4016d9
	github.com/zeromq/goczmq v4.1.0+incompatible
	github.com/zeromq/gomq v0.0.0-20201031135124-cef4e507bb8e
	github.com/zeromq/gomq/zmtp v0.0.0-20201031135124-cef4e507bb8e
	go.etcd.io/etcd v3.3.27+incompatible
	go.uber.org/zap v1.19.1 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	google.golang.org/grpc v1.42.0 // indirect
	google.golang.org/protobuf v1.27.1
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/coreos/bbolt v1.3.6 => go.etcd.io/bbolt v1.3.6

replace google.golang.org/grpc v1.42.0 => google.golang.org/grpc v1.26.0
