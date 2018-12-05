package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"softwareupgrade"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var (
	// BuildDate allows the external linker to set the build date when building
	BuildDate string
)

type (
	// AWSBackupMessage is the message from SNS topic
	AWSBackupMessage struct {
		Version    string    `json:"version"`
		ID         string    `json:"id"`
		DetailType string    `json:"detail-type"`
		Source     string    `json:"source"`
		Account    string    `json:"account"`
		Time       time.Time `json:"time"`
		Region     string    `json:"region"`
		Resources  []string  `json:"resources"`
		Detail     struct {
		} `json:"detail"`
	}
)

func getNetworkID() (result string) {
	result = os.Getenv("NetworkId")
	if result == "" {
		for _, e := range os.Environ() {
			pair := strings.Split(e, "=")
			key := pair[0]
			value := pair[1]
			if strings.ToUpper(key) == "NETWORKID" {
				result = value
			}
		}
	}
	return
}

func getPrintEnv(name string) (result string) {
	result = os.Getenv(name)
	fmt.Printf("%s: %s\n", name, result)
	return
}

func getRegion() (result string) {
	result = os.Getenv("AWS_REGION")
	return
}

// NewConfig creates an empty aws.Config
func NewConfig() (result *aws.Config) {
	return &aws.Config{}
}

// ShowIPAddress shows the IP address
func ShowIPAddress() {
	var sb strings.Builder
	if ifaces, err := net.Interfaces(); err == nil {
		// handle err
		for _, i := range ifaces {
			if addrs, err := i.Addrs(); err == nil {
				// handle err
				for _, addr := range addrs {
					var ip net.IP
					switch v := addr.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}
					if sb.Len() > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(ip.String())
				}
			}
		}
	}
	// process IP address
	if sb.Len() > 0 {
		fmt.Printf("IP: %s\n", sb.String())
	}

}

func makeFilter(NetworkID, role string) (result []*ec2.Filter) {
	result = []*ec2.Filter{
		// This is the NetworkId that I'm interested in
		&ec2.Filter{
			Name:   aws.String("tag:NetworkId"),
			Values: aws.StringSlice([]string{NetworkID}),
		},
		// These are the roles that are running
		&ec2.Filter{
			Name: aws.String("tag:Role"),
			Values: aws.StringSlice([]string{
				role,
			}),
		},
		// Look at only running instances
		&ec2.Filter{
			Name:   aws.String("instance-state-name"),
			Values: aws.StringSlice([]string{"running"}),
		},
	}
	return
}

// Returns the nodes by order of definition
func getFilteredResponse(responses [3]*ec2.DescribeInstancesOutput, nodetypes []string, errs [3]error) (response *ec2.DescribeInstancesOutput, nodetype string, err error) {
	for i := 0; i < 3; i++ {
		if errs[i] == nil && responses[i] != nil && len(responses[i].Reservations) > 0 {
			return responses[i], nodetypes[i], errs[i]
		}
	}
	return nil, "", errors.New("No instances")
}

// BackupHandler is the handler for AWS Lambda
func BackupHandler(ctx context.Context, snsEvent events.SNSEvent) {
	defer func() { fmt.Println("Terminated.") }()
	NetworkID := getNetworkID()
	t := time.Now()
	if BuildDate != "" {
		fmt.Println("Compiled on ", BuildDate)
	}
	fmt.Printf("Scheduled backup event for %s at %s\n", NetworkID, t.String())
	ShowIPAddress()

	var (
		sess   *session.Session
		sshKey []byte
	)
	if sess == nil {
		opts := session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}
		if region := getRegion(); region != "" {
			config := NewConfig().WithRegion(region)
			opts.Config = *config
		}
		sess = session.Must(session.NewSessionWithOptions(opts))
	}

	myBucket := getPrintEnv("Bucket")
	myKey := getPrintEnv("Key")
	sshUser := getPrintEnv("SSHUser")
	sshPass := getPrintEnv("SSHPass")

	if sshPass == "" {
		downloader := s3manager.NewDownloader(sess)
		getS3BucketObj := func(downloader *s3manager.Downloader, bucketName, bucketKey string) (result []byte, err error) {
			var n int64
			buffer := []byte{}
			buf := aws.NewWriteAtBuffer(buffer)
			fmt.Printf("Downloading %s %s\n", bucketName, bucketKey)
			if n, err = downloader.Download(buf, &s3.GetObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(bucketKey),
			}); err == nil {
				result = buf.Bytes()
				fmt.Printf("Bytes read: %v for bucket: %s, key: %s\n", n, bucketName, bucketKey)
			} else {
				fmt.Printf("Bucket: %s, Item: %s, err: %v\n", bucketName, bucketKey, err)
			}
			return
		}

		if encryptedData, err := getS3BucketObj(downloader, myBucket, myKey); err == nil {
			input := &kms.DecryptInput{
				CiphertextBlob: encryptedData,
			}
			kmssvc := kms.New(sess)
			result, err := kmssvc.Decrypt(input)
			if err != nil {
				fmt.Println("Decryption failed.")
				handleErr(err)
				return
			}
			sshKey = result.Plaintext
			fmt.Printf("Decryption succeeded, length: %d\n", len(sshKey))
		} else {
			fmt.Printf("Unable to retrieve S3 bucket: %s key: %s due to error: %v\n", myBucket, myKey, err)
			return
		}
	}

	svc := ec2.New(sess)

	var (
		filters   [3][]*ec2.Filter
		responses [3]*ec2.DescribeInstancesOutput
		errs      [3]error
	)
	filterNames := []string{"Validator", "Maker", "Observer"}
	for i, filterName := range filterNames {
		filters[i] = makeFilter(NetworkID, filterName)
		filtersInput := &ec2.DescribeInstancesInput{Filters: filters[i]}
		responses[i], errs[i] = svc.DescribeInstances(filtersInput)
	}

	fmt.Println("Retrieving instances...")
	if resp, nodetype, err := getFilteredResponse(responses, filterNames, errs); err == nil {
		numInst := len(resp.Reservations)
		lucky := rand.Intn(numInst)
		idx := lucky

		fmt.Printf("Number of instances: %d, chose %d, node type: %s\n", numInst, idx+1, nodetype)

		for _, inst := range resp.Reservations[idx].Instances {
			DNSName := aws.StringValue(inst.PublicDnsName)
			InstanceID := aws.StringValue(inst.InstanceId)
			fmt.Printf("Instance: %s, DNS: %s\n", InstanceID, DNSName)

			var sshconfig *softwareupgrade.SSHConfig
			if sshPass != "" {
				sshconfig = softwareupgrade.NewSSHConfigPassword(sshUser, sshPass, DNSName)
			} else {
				sshconfig = softwareupgrade.NewSSHConfigKeyBytes(sshUser, sshKey, DNSName)
			}
			if err := sshconfig.Connect(); err != nil {
				fmt.Printf("Unable to connect to %s due to error: %v\n", DNSName, err)
				continue
			} else {
				fmt.Printf("Connected to instance: %s\n", DNSName)
			}
			backupCommand := "/opt/quorum/bin/backup-chain-data.py backup"
			pythonCommand := fmt.Sprintf("/usr/bin/python %s", backupCommand)
			cmd := fmt.Sprintf(`screen -d -m %s`, pythonCommand)
			fmt.Printf("Executing %s\n", cmd)
			result, err := sshconfig.Run(cmd)
			fmt.Printf("cmd: '%s', result: '%s', err: '%v'\n", cmd, result, err)
			sshconfig.Destroy()
		}

		fmt.Println("Instances retrieval completed.")
	} else {
		fmt.Printf("Error retrieving instances: %v\n", err)
	}

}

// DebugEnvironment returns true if the environment indicates it is run in debug mode
func DebugEnvironment() (result bool) {
	funcName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	vsCodeLogs := os.Getenv("VSCODE_LOGS")
	result = funcName == "" || vsCodeLogs != "" // is in DebugEnvironment if AWS_LAMBDA_FUNCTION_NAME is empty
	return
}

func main() {
	lambda.Start(BackupHandler)
}
