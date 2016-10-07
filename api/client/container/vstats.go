package container

import (
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
)

func RunvStats(ctx context.Context,dockerCli *client.DockerCli,containers []string, noStream bool, closeChan chan error) error{		
	vStats := vstats{}
	for _,name:=range containers{
		s := &containerDetails{container: name}
		if vStats.add_v(s){
			//collects list of volumes for each container
			s.CollectVol(ctx,dockerCli.Client())
		}	
	}	
	go func(){
	for{	
		vStats.mu.Lock()
		for _,s:=range vStats.c{
			if s.err != nil{
				fmt.Println(s.err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			//collects volume stats for each volume
			for _,u := range s.vs{
				u.CollectVolStats(ctx,dockerCli.Client(),s)
			}
		}
		vStats.mu.Unlock()
		time.Sleep(300*time.Millisecond)
		if noStream{
			break
		}
	}
	}()//go
	close(closeChan)
	var errs []string
	vStats.mu.Lock()
	for _, c := range vStats.c {
		if c.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.container, c.err))
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
	for range time.Tick(500 * time.Millisecond) {
		vStats.mu.Lock()
		printHeader()
		toRemove := []string{}
		for _, s := range vStats.c {
			for _,u := range s.vs{
				if err := u.DisplayVol(s); err != nil && !noStream{
					logrus.Debugf("stats: got error for %s: %v", s.container, err)
					if err == io.EOF {
						toRemove = append(toRemove, s.container)
					}
				}
			}
		}
		for _, name := range toRemove {
			vStats.remove_v(name)
		}
		if len(vStats.c) == 0{
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
