package container

import (
	"fmt"
	"sync"
	"sort"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
)
//volStats is a struct for storing a standard set of stats (future use)
//type volStats struct{}
//	readLat		string
//	writeLat	string
//	avgReadLat	string
//	avgWriteLat	string
//	avgReadReqPers	string
//	avgWriteReqPers	string
//	readOuts	string
//	writeOuts	string
//	readBlkSize	string
//	writeBlkSize	string
//	vmu		sync.Mutex
//	verr		error
//}

type volumeStats struct{
	mu		sync.Mutex
	container	string
	driver		[]string//driver type for each volume
	volumes		[]string//for more than one volume per container
	volumeStats	[]map[string]interface{}//storing volume stats for all volumes as an arra of maps
	err		error
}

type vstats struct{
	mu		sync.Mutex
	vs		[]*volumeStats
}

func (s *vstats) add_v(vs *volumeStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.isKnownContainer_v(vs.container); !exists {
		s.vs = append(s.vs, vs)
		return true
	}
	return false
}

func (s *vstats) remove_v(id string) {
	s.mu.Lock()
	if i, exists := s.isKnownContainer_v(id); exists {
		s.vs = append(s.vs[:i], s.vs[i+1:]...)
	}
	s.mu.Unlock()
}

func (s *vstats) isKnownContainer_v(cid string) (int, bool) {
	for i, c := range s.vs {
		if c.container == cid {
			return i, true
		}
	}
	return -1, false
}
func (s *volumeStats) CollectVol(ctx context.Context,cli client.APIClient){
	logrus.Debugf("collecting volume names for container %s",s.container)
	var getFirst bool
	defer func() {
		if !getFirst {
			getFirst = true
		}
	}()
	volList, err := cli.ContainerInspect(ctx, s.container)
	if err != nil{
		s.err = err
		return
	}
	for i:=0;i< len(volList.Mounts);i++{
		s.volumes = append(s.volumes,volList.Mounts[i].Name)//add all the volume names to s.volumes
		s.driver = append(s.driver,volList.Mounts[i].Driver)//add all the volume types
	}
}//CollectVol

func (s *volumeStats) CollectVolStats(ctx context.Context,cli client.APIClient){
	s.volumeStats = make([]map[string]interface{},len(s.volumes))
	for i:=0;i<len(s.volumes);i++{
		response, err := cli.VolumeInspect(ctx, s.volumes[i])
		if (err!=nil){
			s.err = err
			return
		}
		ret,ok:=response.Status["iostats"].(map[string]interface{})
		if ok{
			s.volumeStats[i] = ret
		}
	}
}

func (s *volumeStats) DisplayVol() error{
	for i,_:=range s.volumes{
		name:=" "
		if(len(s.volumes[i])>=12){
			name = s.volumes[i][:12]
		}else{
			name = s.volumes[i]
		}	
		if (s.err!=nil) {
			err:=s.err
			return err
		}
		var keys []string		
		fmt.Println("Container:"+s.container)
		fmt.Println("Volume:"+name)
		fmt.Println("Driver:"+s.driver[i])
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
	}
 	return nil
}//DisplayVol
