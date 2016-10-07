package container

import (
	"sync"

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

type volStats interface{
	CollectVolStats()
	DisplayVol()
}

type containerDetails struct{
	container	string //container name
	volDrivMap	map[string]string//map of volume and its driver for this container
	err		error
	
	vs		[]*volumeStats//stats of all the volumes of this container(for vmdk defined in vmdk_stats.go)
	//include structures for other volume types here
}

type vstats struct{
	mu		sync.Mutex
	//vs		[]*volumeStats
	c		[]*containerDetails
}

func (s *vstats) add_v(con *containerDetails) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.isKnownContainer_v(con.container); !exists {
		s.c = append(s.c, con)
		return true
	}
	return false
}

func (s *vstats) remove_v(id string) {
	s.mu.Lock()
	if i, exists := s.isKnownContainer_v(id); exists {
		s.c = append(s.c[:i], s.c[i+1:]...)
	}
	s.mu.Unlock()
}

func (s *vstats) isKnownContainer_v(cid string) (int, bool) {
	for i, c := range s.c {
		if c.container == cid {
			return i, true
		}
	}
	return -1, false
}

//CollectVol collects volume name and the driver type per container
func (c *containerDetails) CollectVol(ctx context.Context,cli client.APIClient){
	logrus.Debugf("collecting volume names for container %s",c.container)
	var getFirst bool
	defer func() {
		if !getFirst {
			getFirst = true
		}
	}()
	volList, err := cli.ContainerInspect(ctx, c.container)
	if err != nil{
		c.err = err
		return
	}
	c.volDrivMap = make(map[string]string)
	for i:=0;i< len(volList.Mounts);i++{
		name := volList.Mounts[i].Name
		driver := volList.Mounts[i].Driver
		c.volDrivMap[name]=driver
		x := &volumeStats{driver:driver}
		c.vs = append(c.vs,x)
	}
}//CollectVol

