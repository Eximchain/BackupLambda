package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func Test_debug(t *testing.T) {
	network := "84913"
	os.Setenv("NetworkId", network)
	region := "us-east-1"
	os.Setenv("AWS_REGION", region)
	bucket := fmt.Sprintf("quorum-backup-%s-network-%s", region, network)
	os.Setenv("Bucket", bucket)
	os.Setenv("Key", "enc_ssh")
	os.Setenv("SSHUser", "ubuntu")
	var (
		ctx      context.Context
		snsEvent events.SNSEvent
	)
	BackupHandler(ctx, snsEvent)
}

func makeFilterResponse() (filters [3][]*ec2.Filter, responses [3]*ec2.DescribeInstancesOutput) {
	return
}

func Test_VerifyNodeSelection(t *testing.T) {
	var (
		filters   [3][]*ec2.Filter
		errs      [3]error
		responses [3]*ec2.DescribeInstancesOutput
		NetworkID string
	)

	filterNames := []string{"Validator", "Maker", "Observer"}
	for i, filterName := range filterNames {
		filters[i] = makeFilter(NetworkID, filterName)
		responses[i] = &ec2.DescribeInstancesOutput{}
		responses[i].Reservations = make([]*ec2.Reservation, 1)
		errs[i] = nil
	}
	_, nodetype, _ := getFilteredResponse(responses, filterNames, errs)
	if nodetype != "Validator" {
		t.Fatal("Failed to select Validator")
	}

	filterNames = []string{"Maker", "Observer"}
	for i, filterName := range filterNames {
		filters[i] = makeFilter(NetworkID, filterName)
		responses[i] = &ec2.DescribeInstancesOutput{}
		responses[i].Reservations = make([]*ec2.Reservation, 1)
		errs[i] = nil
	}
	_, nodetype, _ = getFilteredResponse(responses, filterNames, errs)
	if nodetype != "Maker" {
		t.Fatal("Failed to select Maker")
	}

	filterNames = []string{"Observer"}
	for i, filterName := range filterNames {
		filters[i] = makeFilter(NetworkID, filterName)
		responses[i] = &ec2.DescribeInstancesOutput{}
		responses[i].Reservations = make([]*ec2.Reservation, 1)
		errs[i] = nil
	}
	_, nodetype, _ = getFilteredResponse(responses, filterNames, errs)
	if nodetype != "Observer" {
		t.Fatal("Failed to select Observer")
	}

}
