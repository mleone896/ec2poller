package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var (
	dataFile = flag.String("file", "store.json", "data store file name")
	status   = flag.String("status", "stopped|pending|terminated", "the status you would like to poll")
)

var store *StatusStore

type Conn struct {
	aw2  *ec2.EC2
	data map[string]string
}

// create a struct to map toml config file
// TODO: add this as an switch in init()
type AwsConfig struct {
	AwsSecretKey string `toml:"AWS_ACCESS_KEY_ID"`
	AwsAccessKey string `toml:"AWS_SECRET_ACCESS_KEY"`
	Region       string `toml:"AWS_REGION"`
}

func recieveStatus(dataMap map[string]string) <-chan string {
	c := make(chan string)
	go func() {
		for _, v := range dataMap {
			c <- fmt.Sprintf("%s", v)
			//			time.Sleep(time.Duration(rand.Intn(1e3)) * time.Millisecond)
		}
	}()
	return c

}

func (c *Conn) iterateResToMap(resp *ec2.DescribeInstancesOutput) {
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

func (c *Conn) GetEc2Data() {

	resp, err := c.aw2.DescribeInstances(nil)
	if err != nil {
		log.Fatal(err)
	}

	c.iterateResToMap(resp)

}

func NewEc2() *Conn {
	c := new(Conn)
	c.aw2 = ec2.New(&aws.Config{Region: "us-west-2"})

	return c
}

func (d *StatusStore) AddDataToFile(status string, c *Conn) {
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

	// Get a data set to work with
	c.GetEc2Data()

	// lets save some data
	d.AddDataToFile(*status, c)

}
