package container

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

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
	vStats := vstats{}
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


	if opts.v{
		if(len(opts.containers)) == 0{
			fmt.Println("Please provide container name(s)")
			return nil
		}
				
		for _,name:=range opts.containers{
			s := &volumeStats{container: name}
			if vStats.add_v(s){
				//collects list of volumes for each container
				s.CollectVol(ctx,dockerCli.Client())
			}	
		}
		go func(){
		for{	
			vStats.mu.Lock()
			for _,s:=range vStats.vs{
				if s.err != nil{
					fmt.Println(s.err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				//collects volume stats for each volume
				s.CollectVolStats(ctx,dockerCli.Client())
			}
			vStats.mu.Unlock()
			time.Sleep(300*time.Millisecond)
			if opts.noStream{
				break
			}
		}
		}()//go
		close(closeChan)
		var errs []string
		vStats.mu.Lock()
		for _, c := range vStats.vs {
			if c.err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", c.container, c.err))
			}
		}
		vStats.mu.Unlock()
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, ", "))
		}
		printHeader := func() {
			if !opts.noStream{
			  	fmt.Fprint(dockerCli.Out(), "\033[2J")
				fmt.Fprint(dockerCli.Out(), "\033[H")
			}
		
		}
		for range time.Tick(500 * time.Millisecond) {
			vStats.mu.Lock()
			printHeader()
			toRemove := []string{}
			for _, s := range vStats.vs {
				if err := s.DisplayVol(); err != nil && !opts.noStream{
					logrus.Debugf("stats: got error for %s: %v", s.container, err)
					if err == io.EOF {
						toRemove = append(toRemove, s.container)
					}
				}
			}
		for _, name := range toRemove {
			vStats.remove_v(name)
		}
		if len(vStats.vs) == 0{
			vStats.mu.Unlock()
			return nil
		}
		vStats.mu.Unlock()
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
		io.WriteString(w, "\nCONTAINER\tCPU %\tMEM USAGE / LIMIT\tMEM %\tNET I/O\tBLOCK I/O\tPIDS\n")
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
