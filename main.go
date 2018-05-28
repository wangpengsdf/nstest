package main

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"github.com/urfave/cli"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	app := cli.NewApp()
	app.Name = "Dns Test App"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "dns,d",
			Value: "114.114.114.114",
			Usage: "The Address Of Dns Server",
		},
		cli.StringFlag{
			Name:  "time,t",
			Value: "100ms",
			Usage: "Time Interval Of Sending A Dns Message",
		},
		cli.StringFlag{
			Name:  "network,n",
			Value: "udp",
			Usage: "Use UDP Or TCP To Resolve DNS ",
		},
		cli.Int64Flag{
			Name:  "count,c",
			Value: math.MaxInt64,
			Usage: "Max Dns Message Count",
		},
		cli.StringFlag{
			Name:  "file,f",
			Value: "",
			Usage: "The Filename Contains Hosts Names",
		},
	}
	app.Action = func(c *cli.Context) error {
		hostsfile := c.String("file")
		hosts := make([]string, 0)
		if hostsfile != "" {
			buf, err := ioutil.ReadFile(hostsfile)
			if err != nil {
				return err
			}
			hosts = append(hosts, strings.Split(string(buf), "\n")...)
		}
		hosts = append(hosts, []string(c.Args())...)
		if len(hosts) == 0 {
			return errors.New("need Hostname")
		}
		network := strings.ToLower(c.String("network"))
		if network != "tcp" && network != "udp" {
			return errors.New("bad Network Type")
		}
		intval, err := time.ParseDuration(c.String("time"))
		if err != nil {
			return err
		}
		if intval == 0 {
			intval = time.Millisecond * 100
		}
		server := c.String("dns")
		if ip := net.ParseIP(server); ip == nil {
			return errors.New("bad DNS Server")
		}
		sendcount := c.Int64("count")
		var totalnum int64 = 0
		var errnum int64 = 0
		var maxDuration time.Duration = 0
		var minDuration = time.Hour
		totalDuration := time.Duration(0)
		ticker := time.NewTicker(intval)
		stopchan := make(chan os.Signal, 1)
		signal.Notify(stopchan)
		var wg sync.WaitGroup
		for {
			select {
			case <-ticker.C:
				if atomic.LoadInt64(&sendcount) == 0 {
					ticker.Stop()
					stopchan <- syscall.SIGINT
					continue
				}
				id := atomic.AddInt64(&sendcount, -1)
				wg.Add(1)
				go func(idx int64) {
					c := dns.Client{
						Net:     network,
						Timeout: time.Second * 5,
					}
					singleHost := hosts[rand.Intn(len(hosts))]
					m := dns.Msg{}
					m.SetQuestion(singleHost+".", dns.TypeA)
					result, t, err := c.Exchange(&m, server+":53")
					action := fmt.Sprintf("Packet %d %s Resolve %s", idx, server, singleHost)
					atomic.AddInt64(&totalnum, 1)
					if err != nil {
						atomic.AddInt64(&errnum, 1)
						fmt.Printf("%s err: %v\n", action, err)
					} else {
						if len(result.Answer) == 0 {
							atomic.AddInt64(&errnum, 1)
							fmt.Printf("%s no result\n", action)
						} else {
							totalDuration += t
							if t > maxDuration {
								maxDuration = t
							}
							if t < minDuration {
								minDuration = t
							}
						}
					}
					wg.Done()
				}(id)
			case <-stopchan:
				wg.Wait()
				fmt.Println()
				fmt.Printf("Dns Server: %s \n", server)
				fmt.Printf("Network: %s \n", strings.ToUpper(network))
				fmt.Printf("Total Message Num: %d \n", totalnum)
				if totalnum-errnum != 0 {
					fmt.Printf("Average Delay: %s\n", totalDuration/time.Duration(totalnum-errnum))
					fmt.Printf("Minimum Delay: %s\n", minDuration)
					fmt.Printf("Maximum Delay: %s\n", maxDuration)
				}
				fmt.Printf("Err Message Num: %d \n", errnum)
				fmt.Printf("Err Message Percent: %.6f%% \n", float64(errnum*100)/float64(totalnum))
				return nil
			}
		}
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
