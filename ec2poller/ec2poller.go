package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/mleone896/flowpush"
)

var (
	dataFile = flag.String("file", "store.json", "data store file name")
	status   = flag.String("status", "stopped|pending|terminated", "the status you would like to poll")
	useToml  = flag.Bool("toml", true, "Get flowdock creds from toml config")
)

var store *StatusStore

type Conn struct {
	aw2      *ec2.EC2
	data     map[string]string
	chandata chan ec2record
}

type ec2record struct {
	key, status string
}

// create a struct to map toml config file
// TODO: add this as an switch in init()
type FlowDockConfig struct {
	ApiToken         string
	FlowdockAPIToken string
}

func (c *Conn) SetEc2Data(resp *ec2.DescribeInstancesOutput) {
	insMap := make(map[string]string)
	for idx, _ := range resp.Reservations {
		for _, inst := range resp.Reservations[idx].Instances {
			// fmt.Printf("   Instance State: %v InstanceID: %v \n", *inst.State.Name, *inst.InstanceID)
			// dereference pointer
			var id, state string
			id = *inst.PrivateDNSName
			state = *inst.State.Name
			insMap[id] = state
		}
	}
	c.data = insMap
}

func (c *Conn) IterateMapToChan(status string) {

	go func() {
		for k, v := range c.data {

			if v == status {

				c.chandata <- ec2record{k, v}
			}
		}
	}()

}

func (c *Conn) GetEc2Data() {

	resp, err := c.aw2.DescribeInstances(nil)
	if err != nil {
		log.Fatal(err)
	}

	c.SetEc2Data(resp)

}

func (c *Conn) RunLoop(d *StatusStore, config *FlowDockConfig) bool {

	RefreshData(d, c)
	// begin channel operations
	for {
		// set timeout for loop
		timeout := time.After(5 * time.Second)
		// set initial dataset
		select {
		case result := <-c.chandata:
			message := fmt.Sprintf("Status changed to %s for instance %s", result.status, result.key)
			if result.status == "stopped" {
				flowpush.PushMessageToFlowWithKey(config.FlowdockAPIToken, message, "ec2poller")
			} else if result.status == "terminated" {
				flowpush.PushMessageToFlowWithKey(config.FlowdockAPIToken, message, "ec2poller")
			}

		case <-timeout:
			fmt.Println("we hit the timeout\n")
			return false

		}
	}

}

func (c *Conn) Start(d *StatusStore, config *FlowDockConfig) {

	for {

		if c.RunLoop(d, config) {
			fmt.Println("starting another round with refreshed data")
			c.RunLoop(d, config)
		}

	}

}

func RefreshData(d *StatusStore, c *Conn) {
	c.GetEc2Data()
	c.IterateMapToChan(*status)
}

func Add(value string) {
	key := store.Put(value)
	fmt.Println(key)
}

func NewEc2() *Conn {
	c := new(Conn)
	c.aw2 = ec2.New(&aws.Config{Region: "us-west-2"})
	c.chandata = make(chan ec2record)

	return c
}

func main() {
	// Create an EC2 service object in the "us-west-2" region
	// Note that you can also configure your region globally by
	// exporting the AWS_REGION environment variable
	flag.Parse()

	var config FlowDockConfig
	if _, err := toml.DecodeFile("/home/mleone/credentials.toml", &config); err != nil {
		// handle error cause you know i don't trust third party libs
		log.Fatal("something went terribly wrong loading toml")
	}

	// instantiate new ec2 "object"
	c := NewEc2()

	// Get new Status store
	d := NewStatusStore(*dataFile)

	// start the endless loop
	// TODO: need to load data from data store if exists
	// TODO: Need to remove the line from datastore if it is no longer in the status state

	// This catches CTRL-C since this runs in a loop forever
	k := make(chan os.Signal, 1)
	signal.Notify(k, os.Interrupt)
	go func() {
		<-k
		fmt.Printf("Error: user interrupt\n.")
		os.Create(*dataFile)
		d.RemoveOldRecords(c.data, *status)
		d.DataToFile(*status, c)
		os.Exit(-1)
	}()

	c.Start(d, &config)

}
