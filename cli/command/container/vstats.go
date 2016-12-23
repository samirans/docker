package container

import (
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/net/context"
	"github.com/Sirupsen/logrus"
	"github.com/docker/cli/command/container"
	"github.com/docker/cli/command"
)

var VstatsMap = make (map[string]volStats)

func RunvStats(ctx context.Context,dockerCli *client.DockerCli,containers []string, noStream bool, closeChan chan error) error{		
	vStats := vstats{}
	for _,name:=range containers{
		s := &containerVolumes{Name: name}
		if vStats.add(s){
			//collects list of volumes for each container
			s.InitVol(ctx,dockerCli.Client())
		}	
	}	
	go func(){
	for{	
		vStats.mu.Lock()
		for _,s:=range vStats.cs{
			if s.err != nil{
				fmt.Println(s.err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			//collects volume stats for each container
			s.CollectVolStats(ctx,dockerCli.Client())
		}
		vStats.mu.Unlock()
		time.Sleep(5000*time.Millisecond)
		if noStream{
			break
		}
	}
	}()//go
	close(closeChan)
	var errs []string
	vStats.mu.Lock()
	for _, c := range vStats.cs {
		if c.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Name, c.err))
		}
	}
	vStats.mu.Unlock()
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, ", "))
	}
	printHeader := func() {
		if !noStream{
		  	fmt.Fprint(dockerCli.Out(), "\033[2J")
			fmt.Fprint(dockerCli.Out(), "\033[H")
		}
	
	}
	for range time.Tick(5000 * time.Millisecond) {
		vStats.mu.Lock()
		printHeader()
		toRemove := []string{}
		for _, s := range vStats.cs {
			if err := s.DisplayVolStats(); err != nil && !noStream{
				logrus.Debugf("stats: got error for %s: %v", s.Name, err)
				if err == io.EOF {
					toRemove = append(toRemove, s.Name)
				}
			}
		}
		for _, name := range toRemove {
			vStats.remove(name)
		}
		if len(vStats.cs) == 0{
			vStats.mu.Unlock()
			return nil
		}
		vStats.mu.Unlock()
		if noStream {
			break
		}
		select {
			case err, ok := <-closeChan:
				if ok {
					if err != nil {
						// this is suppressing "unexpected EOF" in the cli when the
						// daemon restarts so it shutdowns cleanly
						if err == io.ErrUnexpectedEOF {
							return nil
						}
						return err
					}
				}
			default:
			// just skip
		}
	}
	return nil
}
