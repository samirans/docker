package container

import (
	"fmt"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type vmdkStats struct{
}

func VmdkStatsHandler() *vmdkStats{
	var a vmdkStats
	return &a
}

func (s *vmdkStats) CollectStats(ctx context.Context,cli client.APIClient,vstats *volumeStats){
	response, err := cli.VolumeInspect(ctx,vstats.VolName)
	if (err!=nil){
		fmt.Println(err)
	}
	ret,ok:=response.Status["iostats"].(map[string]interface{})
	if ok{
		vstats.AvgRdsPerSec = ret["avgRd/s"].(string)
		vstats.AvgWrsPerSec = ret["avgWr/s"].(string)
		vstats.AvgInProgRds = ret["avgInProgRds"].(string)
		vstats.AvgInProgWrs = ret["avgInProgWrs"].(string)
		vstats.AvgRdLat	    = ret["avgRdLat(ms)"].(string)
		vstats.AvgWrLat	    = ret["avgWrLat(ms)"].(string)
		vstats.AvgRdReqSz   = ret["avgRdRqSz(bytes)"].(string)
		vstats.AvgWrReqSz   = ret["avgWrRqSz(bytes)"].(string)
		vstats.RdLatency    = ret["rdLat(µs)"].(string)
		vstats.WrLatency    = ret["wrLat(µs)"].(string)
		vstats.RdRate	    = ret["volRdRate(KBps)"].(string)
		vstats.WrRate	    = ret["volWrRate(KBps)"].(string)
	}
}
