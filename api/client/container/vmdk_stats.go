package container

import (
	"fmt"
	"sort"

	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
)

type vmdkStats struct{
        err             error
	volumeStats	map[string]map[string]interface{}
}

func NewVmdkStats() *vmdkStats{
	a := &vmdkStats{err: nil}
	return a
}

func (s *vmdkStats) CollectStats(ctx context.Context,cli client.APIClient,v string){
	response, err := cli.VolumeInspect(ctx,v)
	if (err!=nil){
		s.err = err
		return
	}
	ret,ok:=response.Status["iostats"].(map[string]interface{})
	if ok{
		if s.volumeStats == nil {
			s.volumeStats = make(map[string]map[string]interface{})
		}
		s.volumeStats[v] = ret
	}
}

func (s *vmdkStats) Flush(contName string,driver string,volume string) error{
        fmt.Println("Container:"+contName)
	fmt.Println("Driver:"+driver)
        vname:=" "
        if(len(volume)>=12){
		vname = volume[:12]
	}else{
		vname = volume
	}
	if (s.err!=nil) {
		err:=s.err
		return err
	}
	var keys []string
	fmt.Println("Volume:"+vname)
	for j,_ := range s.volumeStats[volume]{
		keys = append(keys,j)
	}
	sort.Strings(keys)
	for _,k := range keys{
		fmt.Printf("%-14.13s",k)
	}
	fmt.Print("\n")
	for _,val:=range keys{
		fmt.Printf("%-14.13s",s.volumeStats[volume][val].(string))
	}
	fmt.Print("\n")
	return nil
}//DisplayVol
