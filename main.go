package main

import (
	"Pet2/httpMultiplexer"
)

func main() {
	Serv := httpMultiplexer.NewhttpMultiplexer("1234", 5, 1)
	Serv.Init(100, 4, 20)
}
