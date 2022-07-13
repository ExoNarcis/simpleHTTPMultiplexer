package main

import (
	"Pet2/httpMultiplexer"
)

func main() {
	Serv := httpMultiplexer.NewhttpMultiplexer("3333", 10, 1)
	Serv.Init(100, 4, 20, true)
}
