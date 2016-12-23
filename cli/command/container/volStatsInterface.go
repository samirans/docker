package container

import(
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type volStats interface{
        CollectStats(context.Context,client.APIClient,*volumeStats)
}



