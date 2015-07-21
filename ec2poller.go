package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var (
	dataFile = flag.String("file", "store.json", "data store file name")
	status   = flag.String("status", "stopped|pending|terminated", "the status you would like to poll")
	useToml  = flag.Bool("toml", false, "A switch to use creds from tomlfile instead of ENV")
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
type AwsConfig struct {
	AwsSecretKey string `toml:"AWS_ACCESS_KEY_ID"`
	AwsAccessKey string `toml:"AWS_SECRET_ACCESS_KEY"`
	Region       string `toml:"AWS_REGION"`
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

func NewEc2() *Conn {
	c := new(Conn)
	c.aw2 = ec2.New(&aws.Config{Region: "us-west-2"})
	c.chandata = make(chan ec2record)

	return c
}

func (d *StatusStore) DataToFile(status string, c *Conn) {
	for k, v := range c.data {
		if v == status {
			if _, ok := d.status[k]; ok {
			} else {
				err := d.save(k, v)
				if err != nil {
					log.Printf("something went wrong save %s", k)
				}
			}
		}
	}
}

func (c *Conn) RunLoop(d *StatusStore) bool {

	RefreshData(d, c)
	// begin channel operations
	for {
		// set timeout for loop
		timeout := time.After(5 * time.Second)
		// set initial dataset
		select {
		case result := <-c.chandata:
			fmt.Println(result.key)

		case <-timeout:
			fmt.Println("we hit the timeout\n")
			return false

		}
	}

}

func (c *Conn) Start(d *StatusStore) {

	for {

		if c.RunLoop(d) {
			fmt.Println("starting another round with refreshed data")
			c.RunLoop(d)
		}

	}

}

func RefreshData(d *StatusStore, c *Conn) {
	c.GetEc2Data()
	//d.DataToFile(*status, c)
	c.IterateMapToChan(*status)
}

func Add(value string) {
	key := store.Put(value)
	fmt.Println(key)
}

func main() {
	// Create an EC2 service object in the "us-west-2" region
	// Note that you can also configure your region globally by
	// exporting the AWS_REGION environment variable
	flag.Parse()

	// instantiate new ec2 "object"
	c := NewEc2()

	// Get new Status store
	d := NewStatusStore(*dataFile)

	// start the endless loop
	// TODO: need to load data from data store if exists
	// TODO: need to save chan data into data store on exit / CTRL-C sig

	c.Start(d)

}
