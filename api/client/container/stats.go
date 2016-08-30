package container

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
//	"encoding/json"
	
	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/system"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/filters"
	"github.com/spf13/cobra"


)

type statsOptions struct {
	all      bool
	noStream bool
	v	 bool


	containers []string
}

// NewStatsCommand creates a new cobra.Command for `docker stats`
func NewStatsCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts statsOptions

	cmd := &cobra.Command{
		Use:   "stats [OPTIONS] [CONTAINER...]",
		Short: "Display a live stream of container(s) resource usage statistics",
		Args:  cli.RequiresMinArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runStats(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.all, "all", "a", false, "Show all containers (default shows just running)")
	flags.BoolVar(&opts.noStream, "no-stream", false, "Disable streaming stats and only pull the first result")
	flags.BoolVarP(&opts.v, "volume", "v", false, "Get volume stats")

	return cmd
}

// runStats displays a live stream of resource usage statistics for one or more containers.
// This shows real-time information on CPU usage, memory usage, and network I/O.
func runStats(dockerCli *client.DockerCli, opts *statsOptions) error {
	showAll := len(opts.containers) == 0
	closeChan := make(chan error)

	ctx := context.Background()

	// monitorContainerEvents watches for container creation and removal (only
	// used when calling `docker stats` without arguments).
	monitorContainerEvents := func(started chan<- struct{}, c chan events.Message) {
		f := filters.NewArgs()
		f.Add("type", "container")
		options := types.EventsOptions{
			Filters: f,
		}
		resBody, err := dockerCli.Client().Events(ctx, options)
		// Whether we successfully subscribed to events or not, we can now
		// unblock the main goroutine.
		close(started)
		if err != nil {
			closeChan <- err
			return
		}
		defer resBody.Close()

		system.DecodeEvents(resBody, func(event events.Message, err error) error {
			if err != nil {
				closeChan <- err
				return nil
			}
			c <- event
			return nil
		})
	}

	// waitFirst is a WaitGroup to wait first stat data's reach for each container
	waitFirst := &sync.WaitGroup{}

	cStats := stats{}
	// getContainerList simulates creation event for all previously existing
	// containers (only used when calling `docker stats` without arguments).
	getContainerList := func() {
		options := types.ContainerListOptions{
			All: opts.all,
		}
		cs, err := dockerCli.Client().ContainerList(ctx, options)
		if err != nil {
			closeChan <- err
		}
		for _, container := range cs {
			s := &containerStats{Name: container.ID[:12]}
			if cStats.add(s) {
				waitFirst.Add(1)
				go s.Collect(ctx, dockerCli.Client(), !opts.noStream, waitFirst)
			}
		}
	}



	if showAll {
		// If no names were specified, start a long running goroutine which
		// monitors container events. We make sure we're subscribed before
		// retrieving the list of running containers to avoid a race where we
		// would "miss" a creation.
		started := make(chan struct{})
		eh := system.InitEventHandler()
		eh.Handle("create", func(e events.Message) {
			if opts.all {
				s := &containerStats{Name: e.ID[:12]}
				if cStats.add(s) {
					waitFirst.Add(1)
					go s.Collect(ctx, dockerCli.Client(), !opts.noStream, waitFirst)
				}
			}
		})

		eh.Handle("start", func(e events.Message) {
			s := &containerStats{Name: e.ID[:12]}
			if cStats.add(s) {
				waitFirst.Add(1)
				go s.Collect(ctx, dockerCli.Client(), !opts.noStream, waitFirst)
			}
		})

		eh.Handle("die", func(e events.Message) {
			if !opts.all {
				cStats.remove(e.ID[:12])
			}
		})

		eventChan := make(chan events.Message)
		go eh.Watch(eventChan)
		go monitorContainerEvents(started, eventChan)
		defer close(eventChan)
		<-started

		// Start a short-lived goroutine to retrieve the initial list of
		// containers.
		getContainerList()
	} 

	




//=================edit
	if opts.v{

	waitFirst.Add(1)
	//fmt.Println(system.GetVols(dockerCli,opts.containers,waitFirst))
	volMap := make(map[int][]string)
	volMap= system.GetVols(dockerCli,opts.containers,waitFirst)
	//for i:=0;i<len(volMap);i++{
					 //fmt.Println(volMap[i])
	//}
	//format:="%s\t%s\t%s"
	fmt.Fprint(dockerCli.Out(), "\033[2J")
        fmt.Fprint(dockerCli.Out(), "\033[H")

	for i:=0;i<len(volMap);i++{

	w := tabwriter.NewWriter(dockerCli.Out(), 5, 1, 3, ' ', 0)
	fmt.Println("")
	fmt.Println("CONTAINER:"+volMap[i][0])
	io.WriteString(w,"VOLUME NAME\tDRIVER NAME\tREAD LATENCY(us)\tWRITE LATENCY(us)\t\n")

	for j:=1;j<len(volMap[i]);j++{
		response,err:=system.GetVolStats(dockerCli,volMap[i][j])

		md,ok := response.Status["iostats"].(map[string]interface{})

		readLat,okread:= md["read_latency(us)"].(map[string]interface{})
		writeLat,okwrite:= md["write_latency(us)"].(map[string]interface{})
		
                        name:=" "

                        if(len(response.Name)>=12){
                        name = response.Name[:12]
                        }else{
                        name = response.Name
                        }

		
			format := "%s\t%s\t%s\t%s\n"
			if (err!=nil || ok!=true || okread!=true || okwrite!=true) {
				format = "%s\t%s\t%s\t%s\n"
				errStr := "--"
				fmt.Fprintf(w, format,
				name, errStr, errStr,errStr,
				)
				//err := s.err
				//return err
			}else{
			fmt.Fprintf(w, format,
				name,
				response.Driver,
				readLat["value"],
				writeLat["value"],
				)
			}
		}
			w.Flush()
		}

		close(closeChan)



		// Do a quick pause to detect any error with the provided list of
		// container names.
		//time.Sleep(1500 * time.Millisecond)
		var errs []string
		cStats.mu.Lock()
		for _, c := range cStats.cs {
					c.mu.Lock()
					if c.err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", c.Name, c.err))
					}
					c.mu.Unlock()
		}
		cStats.mu.Unlock()
		if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, ", "))
		}
		return nil
	}else{
		// Artificially send creation events for the containers we were asked to
		// monitor (same code path than we use when monitoring all containers).
		for _, name := range opts.containers {
			s := &containerStats{Name: name}
			if cStats.add(s) {
				waitFirst.Add(1)
				go s.Collect(ctx, dockerCli.Client(), !opts.noStream, waitFirst)
			}
		}

		// We don't expect any asynchronous errors: closeChan can be closed.
		close(closeChan)

		// Do a quick pause to detect any error with the provided list of
		// container names.
		time.Sleep(1500 * time.Millisecond)
		var errs []string
		cStats.mu.Lock()
		for _, c := range cStats.cs {
			c.mu.Lock()
			if c.err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", c.Name, c.err))
			}
			c.mu.Unlock()
		}
		cStats.mu.Unlock()
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, ", "))
		}
	}

	// before print to screen, make sure each container get at least one valid stat data
	waitFirst.Wait()

	w := tabwriter.NewWriter(dockerCli.Out(), 20, 1, 3, ' ', 0)
	printHeader := func() {
		if !opts.noStream {
			fmt.Fprint(dockerCli.Out(), "\033[2J")
			fmt.Fprint(dockerCli.Out(), "\033[H")
		}
		io.WriteString(w, "CONTAINER\tCPU %\tMEM USAGE / LIMIT\tMEM %\tNET I/O\tBLOCK I/O\tPIDS\n")
	}

	for range time.Tick(500 * time.Millisecond) {
		printHeader()
		toRemove := []string{}
		cStats.mu.Lock()
		for _, s := range cStats.cs {
			if err := s.Display(w); err != nil && !opts.noStream {
				logrus.Debugf("stats: got error for %s: %v", s.Name, err)
				if err == io.EOF {
					toRemove = append(toRemove, s.Name)
				}
			}
		}
		cStats.mu.Unlock()
		for _, name := range toRemove {
			cStats.remove(name)
		}
		if len(cStats.cs) == 0 && !showAll {
			return nil
		}
		w.Flush()
		if opts.noStream {
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
