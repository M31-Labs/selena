module m31labs.dev/selena

go 1.26

require (
	github.com/odvcencio/gotreesitter v0.15.3
	m31labs.dev/gosx v0.0.0-00010101000000-000000000000
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/odvcencio/turboquant v0.1.3 // indirect
)

replace (
	github.com/odvcencio/gotreesitter => ../gotreesitter
	m31labs.dev/gosx => ../gosx
)
