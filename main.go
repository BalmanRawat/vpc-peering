package main

import (
	"errors"
	. "fmt"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	yaml "gopkg.in/yaml.v2"
)

type Account struct {
	ID      string   `yaml:"account"`
	Region  string   `yaml:"region"`
	Profile string   `yaml:"profile"`
	VPCID   string   `yaml:"vpcId"`
	Subnets []string `yaml:"subnets"`
	meta    map[string]interface{}
}

type Config struct {
	PeeringName string  `yaml:"peering-name"`
	PeeringId   string  `yaml:"peeringId"`
	Requester   Account `yaml:"requester"`
	Accepter    Account `yaml:"accepter"`
}

func (a *Account) printDetails() error {
	//Creating Session from given Profile and Region
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: a.Profile,
		Config: aws.Config{
			Region: aws.String(a.Region),
		},
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil {
		return err
	}
	svc := ec2.New(sess)

	//Getting Details of the VPC
	Println("------------------------------------------------------------")
	Printf("   VPC_ID: %s  ACCOUNT_ID: %s\n", a.VPCID, a.ID)
	Println("------------------------------------------------------------")
	input := &ec2.DescribeVpcsInput{
		VpcIds: []*string{
			aws.String(a.VPCID),
		},
	}
	result, err := svc.DescribeVpcs(input)
	if err != nil {
		return err
	}

	if len(result.Vpcs) == 0 {
		Println("No VPC returned")
		return errors.New("No VPC returned")
	}

	vpc := result.Vpcs[0]
	vpcId := *vpc.VpcId
	name := vpcId
	cidr := *vpc.CidrBlock

	for _, t := range vpc.Tags {
		if *t.Key == "Name" {
			name = *t.Value
		}
	}

	Printf("|%-30s|%-25s|%-20s|\n", "Name", "VpcId", "VPC CIDRs")
	Printf("|%-30s|%-25s|%-20s|\n", name, vpcId, cidr)

	//Getting Details of the Subnets
	sf := &ec2.Filter{
		Name:   aws.String("vpc-id"),
		Values: []*string{&a.VPCID},
	}

	dsi := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{sf},
	}
	dsr, err := svc.DescribeSubnets(dsi)
	if err != nil {
		return err
	}

	subnets := dsr.Subnets
	Printf("\n\n|%-30s|%-25s|%-20s|%-12s|\n", "Name", "SubnetId", "Subnet CIDR", "AZ")
	for _, s := range subnets {
		sId := *s.SubnetId
		a.Subnets = append(a.Subnets, sId)
		name := sId
		for _, t := range s.Tags {
			if *t.Key == "Name" {
				name = *t.Value
			}
		}
		Printf("|%-30s|%-25s|%-20s|%-12s|\n", name, sId, *s.CidrBlock, *s.AvailabilityZone)
	}

	//Getting Details of the All the routes associated with the Subnet
	Printf("\n\n|%-25s|%-25s|%-25s|%s|\n", "RouteTableID", "Associated SubnetId", "Associated SubnetName", "Current Route Destinations")
	for _, s := range subnets {
		sId := *s.SubnetId
		sn := sId
		for _, t := range s.Tags {
			if *t.Key == "Name" {
				sn = *t.Value
			}
		}

		rf := &ec2.Filter{
			Name:   aws.String("association.subnet-id"),
			Values: []*string{s.SubnetId},
		}

		dri := &ec2.DescribeRouteTablesInput{
			Filters: []*ec2.Filter{rf},
		}
		drr, err := svc.DescribeRouteTables(dri)
		if err != nil {
			return err
		}

		if len(drr.RouteTables) == 0 {
			continue
		}
		rt := drr.RouteTables[0]
		rtId := *rt.RouteTableId
		rtDestinations := ""
		comma := ""

		for _, r := range rt.Routes {
			if r.DestinationCidrBlock != nil {
				rtDestinations = rtDestinations + comma + *r.DestinationCidrBlock
				comma = ","
			}
		}
		Printf("|%-25s|%-25s|%-25s|%s|\n", rtId, sId, sn, rtDestinations)
	}

	return nil
}

func main() {

	var c Config
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		Println(err)
		return
	}

	yaml.Unmarshal(data, &c)

	err = c.Requester.printDetails()
	if err != nil {
		Println(err)
		return
	}

	err = c.Accepter.printDetails()
	if err != nil {
		Println(err)
		return
	}

	c.PeeringId = "fake-peering-id"

	y, err := yaml.Marshal(c)
	if err != nil {
		Println(err)
		return
	}
	err = ioutil.WriteFile("/tmp/gen.yaml", y, 0644)
	if err != nil {
		Println(err)
		return
	}

	return
}
