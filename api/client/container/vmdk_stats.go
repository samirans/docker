package container

import (
	"fmt"
	"sort"
	"sync"

	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
)

type volumeStats struct{
	driver		string
        volumeStats     []map[string]interface{}//storing volume stats for all volumes as an array of maps
	mu 		sync.Mutex
        err             error
}


func (s *volumeStats) CollectVolStats(ctx context.Context,cli client.APIClient,cont *containerDetails){
	s.volumeStats = make([]map[string]interface{},len(cont.volDrivMap))
	var i int = 0
	for key,_ := range cont.volDrivMap{
		response, err := cli.VolumeInspect(ctx, key)
		if (err!=nil){
			s.err = err
			return
		}
		ret,ok:=response.Status["iostats"].(map[string]interface{})
		if ok{
			s.volumeStats[i] = ret
			i++
		}
	}
}

func (s *volumeStats) DisplayVol(cont *containerDetails) error{
        var i int = 0
        for k,v := range cont.volDrivMap{
                name:=" "
                if(len(k)>=12){
                        name = k[:12]
                }else{
                        name = k
                }
                if (s.err!=nil) {
                        err:=s.err
                        return err
                }
                var keys []string
                fmt.Println("Container:"+cont.container)
                fmt.Println("Volume:"+name)
                fmt.Println("Driver:"+v)
                for j,_ := range s.volumeStats[i]{
                        keys = append(keys,j)
                }
                sort.Strings(keys)
                for _,k := range keys{
                        fmt.Printf("%-14.13s",k)
                }
                fmt.Print("\n")
                for _,val:=range keys{
                        fmt.Printf("%-14.13s",s.volumeStats[i][val].(string))
                }
                fmt.Print("\n")
                i++
        }
        return nil
}//DisplayVol

