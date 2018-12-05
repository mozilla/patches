package scanners

import (
	"fmt"
	"testing"
	"time"

	"github.com/arcrose/patches/pkg/done"
	"github.com/arcrose/patches/pkg/platform"
	"github.com/arcrose/patches/pkg/vulnerability"
)

type mockSource struct {
	numVulns uint
	numErrs  uint
}

func TestScheduler(t *testing.T) {
	sched := newScheduler(100 * time.Millisecond)

	if err := sched.start(); err != nil {
		t.Errorf("Should have been able to start scheduler but got error '%s'", err.Error())
	}
	if err := sched.start(); err == nil {
		t.Errorf("Should not have been able to start scheduler a second time")
	}

	ticks := sched.clock()
	ticksCounted := 0
	finished := make(chan bool, 1)

	go func() {
		stop := time.Now().Add(350 * time.Millisecond)

	top:
		for {
			timeLeft := stop.Sub(time.Now())

			select {
			case <-ticks:
				ticksCounted++

			case <-time.After(timeLeft):
				finished <- true
				break top
			}
		}
	}()

	<-finished

	if ticksCounted != 3 {
		t.Errorf("Should have read 3 ticks from clock")
	}

	if err := sched.stop(); err != nil {
		t.Errorf("Should have been able to stop scheduler, but got error '%s'", err.Error())
	}
	if err := sched.stop(); err == nil {
		t.Errorf("Should not have been able to stop scheduler a second time")
	}

	select {
	case <-sched.clock():
		t.Errorf("Should not receive clock ticks after stopping scheduler")

	case <-time.After(250 * time.Millisecond):
		break
	}
}

func TestJobRunner(t *testing.T) {
	type config struct {
		TimesToSignal uint
		SignalPause   time.Duration
		VulnsToGen    uint
		ErrsToGen     uint
	}

	type testCase struct {
		Desc string
		Cfg  config
		Fn   func(config, jobRunner, <-chan done.Done) []error
	}

	testCases := []testCase{
		{
			Desc: "Should read all of the vulns and errors produced by the runner",
			Cfg: config{
				TimesToSignal: 1,
				VulnsToGen:    3,
				ErrsToGen:     2,
			},
			Fn: func(cfg config, runner jobRunner, finished <-chan done.Done) []error {
				var vulnsCounted uint = 0
				var errsCounted uint = 0
				errs := []error{}
				stream := runner.start()

			top:
				for {
					select {
					case <-stream.Vulns:
						vulnsCounted++

					case <-stream.Errors:
						errsCounted++

					case <-stream.Finished:
						errs = append(errs, fmt.Errorf("Stream finished unexpectedly"))

					case <-finished:
						break top
					}
				}

				if vulnsCounted != cfg.VulnsToGen {
					errs = append(errs, fmt.Errorf(
						"Expected to get %d vulns, but only got %d",
						cfg.VulnsToGen,
						vulnsCounted))
				}
				if errsCounted != cfg.ErrsToGen {
					errs = append(errs, fmt.Errorf(
						"Expected to get %d errrs, but only got %d",
						cfg.ErrsToGen,
						errsCounted))
				}

				return errs
			},
		},
		/*
			{
				Desc: "Should read everything written by multiple jobs",
			},
			{
				Desc: "Should only read values when a job is running",
			},
		*/
	}

	for caseNum, tcase := range testCases {
		t.Logf("Running TestJobRunner case #%d: %s", caseNum, tcase.Desc)

		signals := make(chan signal, tcase.Cfg.TimesToSignal)
		runner := newJobRunner(
			mockSource{tcase.Cfg.VulnsToGen, tcase.Cfg.ErrsToGen},
			platform.Debian8,
			signals)
		finished := make(chan done.Done, 1)

		finished <- done.Done{}

		errs := tcase.Fn(tcase.Cfg, runner, finished)

		for _, err := range errs {
			t.Error(err)
		}
	}
}

func (mock mockSource) Vulnerabilities(pform platform.Platform) vulnerability.Job {
	vulns := make(chan vulnerability.Vulnerability, mock.numVulns)
	finished := make(chan done.Done, 1)
	errors := make(chan error)

	go func() {
		var i uint
		for i = 0; i < mock.numVulns; i++ {
			vulns <- vulnerability.Vulnerability{}
			fmt.Println("Wrote a vuln")
		}
		for i = 0; i < mock.numErrs; i++ {
			errors <- fmt.Errorf("")
			fmt.Println("Wrote an error")
		}
		finished <- done.Done{}
	}()

	return vulnerability.Job{
		vulns,
		finished,
		errors,
	}
}
