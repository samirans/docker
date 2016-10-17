package container

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
)

type containerVolumes struct {
	Name             string
	DriverVolStats	 map[string][]volumeStats
	mu               sync.Mutex
	err              error
}

type volumeStats struct{
	VolName         string
	AvgRdsPerSec    string
	AvgWrsPerSec    string
	AvgInProgRds    string
	AvgInProgWrs	string
	AvgRdLat        string
	AvgWrLat        string
	AvgRdReqSz      string
	AvgWrReqSz      string
	RdLatency       string
	WrLatency       string
	RdRate          string
	WrRate          string
}

type vstats struct {
	mu sync.Mutex
	cs []*containerVolumes
}

func (s *vstats) add(cs *containerVolumes) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.isKnownContainer(cs.Name); !exists {
		s.cs = append(s.cs, cs)
		return true
	}
	return false
}

func (s *vstats) remove(id string) {
	s.mu.Lock()
	if i, exists := s.isKnownContainer(id); exists {
		s.cs = append(s.cs[:i], s.cs[i+1:]...)
	}
	s.mu.Unlock()
}

func (s *vstats) isKnownContainer(cid string) (int, bool) {
	for i, c := range s.cs {
		if c.Name == cid {
			return i, true
		}
	}
	return -1, false
}

//CollectVol collects volume name and the driver type per container
func (c *containerVolumes) InitVol(ctx context.Context,cli client.APIClient){
        logrus.Debugf("collecting volume names for container %s",c.Name)
        var getFirst bool
        defer func() {
                if !getFirst {
                        getFirst = true
                }
        }()
        containerData, err := cli.ContainerInspect(ctx, c.Name)
        if err != nil{
                c.err = err
                return
        }
	for i:=0;i< len(containerData.Mounts);i++{
                volName := containerData.Mounts[i].Name
                driver := containerData.Mounts[i].Driver
		if c.DriverVolStats == nil{
			c.DriverVolStats = make(map[string][]volumeStats)
		}
		x:=volumeStats{VolName: volName}
		c.DriverVolStats[driver] = append(c.DriverVolStats[driver],x)
	}
}//CollectVol

func (s *containerVolumes)CollectVolStats(ctx context.Context,cli client.APIClient){
	for k,_ := range s.DriverVolStats{
		for i:=0;i<len(s.DriverVolStats[k]);i++{
			if(k=="vmdk"){
				_,ok := VstatsMap[k]
				if !ok{
					vs := VmdkStatsHandler()
					VstatsMap[k] = vs
				}
				VstatsMap[k].CollectStats(ctx,cli,&s.DriverVolStats[k][i])
			}
		}
	}
}

func (s *containerVolumes)DisplayVolStats() error{
	for k,_ := range s.DriverVolStats{
		for i:=0;i<len(s.DriverVolStats[k]);i++{
			fmt.Println(s.DriverVolStats[k][i])	
		}
	}
	return nil
}	
