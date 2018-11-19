package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

func TestSSM(t *testing.T) {
	var (
		sess *session.Session
	)
	if sess == nil {
		sess = session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))
	}
	os.Setenv("AWS_REGION", "us-east-1")
	svcSsm := ssm.New(sess, NewConfig().WithRegion(getRegion()))
	Run(svcSsm, "i-03536925539052e0a", "unknown", "uptime")
}

func TestSSHGenerator(t *testing.T) {
	//Empty string slice to place the byte literals in
	var dataSlice []string

	//This will be the generated go file
	//Note: the path is ./ because we are using the go generate command
	// inside the main.go file
	outfile, err := os.Create("./generatedSSH.go")
	if err != nil {
		panic(err.Error())
	}
	defer outfile.Close()

	//This is the file to turn to []bytes
	//Make sure the name used is the name used to build the sample data
	infile, err := ioutil.ReadFile("/tmp/encrypted-ssh-us-east-1")
	if err != nil {
		panic(err.Error())
	}

	//Write the initial data to the generated go file
	//package main
	//var data = []byte{ EOF... so far
	outfile.Write([]byte("package main\n\nvar (\n\tdata = []byte{"))

	//Here we loop over each byte in the []byte from the sample
	//data file.
	//Take the byte literal and format it as a string
	//Then append it to the empty []string dataSlice we created at
	//the top of the func
	//Depending on the size of infile this could take a bit
	for _, b := range infile {
		bString := fmt.Sprintf("%v", b)
		dataSlice = append(dataSlice, bString)
	}

	//We join the []string together separating it with commas
	//Remember we are writing a go src file so everything has to be a string
	dataString := strings.Join(dataSlice, ", \n")

	//Write all the data to the generated file
	outfile.WriteString(dataString)

	//Finish off the data
	outfile.Write([]byte("}\n)"))
}

func Test_debug(t *testing.T) {
	network := "84913"
	os.Setenv("NetworkId", network)
	region := "ap-south-1"
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
