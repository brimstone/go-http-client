go-http-client
==============

I just wanted to be able to add proxies easier.


```go
import (
	"github.com/brimstone/go-http-client"
)

func(){
	resp, err := http.WithSOCKS5("localhost:9050").Get("https://brimstone.github.io")
}
