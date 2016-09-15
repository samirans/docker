package container

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"sort"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/go-units"
	"golang.org/x/net/context"
)

type containerStats struct {
	Name             string
	CPUPercentage    float64
	Memory           float64
	MemoryLimit      float64
	MemoryPercentage float64
	NetworkRx        float64
	NetworkTx        float64
	BlockRead        float64
	BlockWrite       float64
	PidsCurrent      uint64
	mu               sync.Mutex
	err              error
}

type stats struct {
	mu sync.Mutex
	cs []*containerStats
}

func (s *stats) add(cs *containerStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.isKnownContainer(cs.Name); !exists {
		s.cs = append(s.cs, cs)
		return true
	}
	return false
}

func (s *stats) remove(id string) {
	s.mu.Lock()
	if i, exists := s.isKnownContainer(id); exists {
		s.cs = append(s.cs[:i], s.cs[i+1:]...)
	}
	s.mu.Unlock()
}

func (s *stats) isKnownContainer(cid string) (int, bool) {
	for i, c := range s.cs {
		if c.Name == cid {
			return i, true
		}
	}
	return -1, false
}

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
	}
}//CollectVol

func (s *volumeStats) CollectVolStats(ctx context.Context,cli client.APIClient){
	for i:=0;i<len(s.volumes);i++{
		response, err := cli.VolumeInspect(ctx, s.volumes[i])
		if (err!=nil){
			s.err = err
			return
		}
		ret,ok:=response.Status["iostats"].(map[string]interface{})
		if ok{
			s.volumeStats = append(s.volumeStats,ret)
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


func (s *containerStats) Collect(ctx context.Context, cli client.APIClient, streamStats bool, waitFirst *sync.WaitGroup) {
	logrus.Debugf("collecting stats for %s", s.Name)
	var (
		getFirst       bool
		previousCPU    uint64
		previousSystem uint64
		u              = make(chan error, 1)
	)

	defer func() {
		// if error happens and we get nothing of stats, release wait group whatever
		if !getFirst {
			getFirst = true
			waitFirst.Done()
		}
	}()

	responseBody, err := cli.ContainerStats(ctx, s.Name, streamStats)
	if err != nil {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		return
	}
	defer responseBody.Close()

	dec := json.NewDecoder(responseBody)
	go func() {
		for {
			var v *types.StatsJSON

			if err := dec.Decode(&v); err != nil {
				dec = json.NewDecoder(io.MultiReader(dec.Buffered(), responseBody))
				u <- err
				if err == io.EOF {
					break
					}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			var memPercent = 0.0
			var cpuPercent = 0.0

			// MemoryStats.Limit will never be 0 unless the container is not running and we haven't
			// got any data from cgroup
			if v.MemoryStats.Limit != 0 {
				memPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
			}

			previousCPU = v.PreCPUStats.CPUUsage.TotalUsage
			previousSystem = v.PreCPUStats.SystemUsage
			cpuPercent = calculateCPUPercent(previousCPU, previousSystem, v)
			blkRead, blkWrite := calculateBlockIO(v.BlkioStats)
			s.mu.Lock()
			s.CPUPercentage = cpuPercent
			s.Memory = float64(v.MemoryStats.Usage)
			s.MemoryLimit = float64(v.MemoryStats.Limit)
			s.MemoryPercentage = memPercent
			s.NetworkRx, s.NetworkTx = calculateNetwork(v.Networks)
			s.BlockRead = float64(blkRead)
			s.BlockWrite = float64(blkWrite)
			s.PidsCurrent = v.PidsStats.Current
			s.mu.Unlock()
			u <- nil
			if !streamStats {
				return
			}
		}
	}()
	for {
		select {
		case <-time.After(2 * time.Second):
			// zero out the values if we have not received an update within
			// the specified duration.
			s.mu.Lock()
			s.CPUPercentage = 0
			s.Memory = 0
			s.MemoryPercentage = 0
			s.MemoryLimit = 0
			s.NetworkRx = 0
			s.NetworkTx = 0
			s.BlockRead = 0
			s.BlockWrite = 0
			s.PidsCurrent = 0
			s.err = errors.New("timeout waiting for stats")
			s.mu.Unlock()
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		case err := <-u:
			if err != nil {
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
				continue
			}
			s.err = nil
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		}
		if !streamStats {
			return
		}
	}
}

func (s *containerStats) Display(w io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// NOTE: if you change this format, you must also change the err format below!
	format := "%s\t%.2f%%\t%s / %s\t%.2f%%\t%s / %s\t%s / %s\t%d\n"
	if s.err != nil {
		format = "%s\t%s\t%s / %s\t%s\t%s / %s\t%s / %s\t%s\n"
		errStr := "--"
		fmt.Fprintf(w, format,
			s.Name, errStr, errStr, errStr, errStr, errStr, errStr, errStr, errStr, errStr,
		)
		err := s.err
		return err
	}
	fmt.Fprintf(w, format,
		s.Name,
		s.CPUPercentage,
		units.BytesSize(s.Memory), units.BytesSize(s.MemoryLimit),
		s.MemoryPercentage,
		units.HumanSize(s.NetworkRx), units.HumanSize(s.NetworkTx),
		units.HumanSize(s.BlockRead), units.HumanSize(s.BlockWrite),
		s.PidsCurrent)
	return nil
}

func calculateCPUPercent(previousCPU, previousSystem uint64, v *types.StatsJSON) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CPUStats.CPUUsage.TotalUsage) - float64(previousCPU)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.CPUStats.SystemUsage) - float64(previousSystem)
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return cpuPercent
}

func calculateBlockIO(blkio types.BlkioStats) (blkRead uint64, blkWrite uint64) {
	for _, bioEntry := range blkio.IoServiceBytesRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			blkRead = blkRead + bioEntry.Value
		case "write":
			blkWrite = blkWrite + bioEntry.Value
		}
	}
	return
}

func calculateNetwork(network map[string]types.NetworkStats) (float64, float64) {
	var rx, tx float64

	for _, v := range network {
		rx += float64(v.RxBytes)
		tx += float64(v.TxBytes)
	}
	return rx, tx
}
