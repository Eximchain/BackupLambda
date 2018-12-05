package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func Test_debug(t *testing.T) {
	network := "84913"
	os.Setenv("NetworkId", network)
	region := "ap-south-1"
	os.Setenv("AWS_REGION", region)
	bucket := fmt.Sprintf("quorum-backup-%s-network-%s", region, network)
	os.Setenv("Bucket", bucket)
	os.Setenv("Key", "enc_ssh")
	os.Setenv("SSHUser", "ubuntu")
	os.Setenv("SSHPass", "pass")
	var (
		ctx      context.Context
		snsEvent events.SNSEvent
	)
	BackupHandler(ctx, snsEvent)
}
