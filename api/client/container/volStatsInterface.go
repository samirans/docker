package container

import(
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
)

type volStats interface{
        CollectStats(context.Context,client.APIClient,string)
        Flush(string,string,string) error
}



