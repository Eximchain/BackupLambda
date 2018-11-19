package main

import (
	"context"
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
	"github.com/aws/aws-sdk-go/service/ssm"
)

var (
	BuildDate string
)

type (
	// Message from SNS topic
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

func getNetworkId() (result string) {
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

func NewConfig() (result *aws.Config) {
	return &aws.Config{}
}

func Run(svcSsm *ssm.SSM, instanceId, DNSName, command string) {
	param := make(map[string][]*string)
	param["commands"] = aws.StringSlice([]string{command})
	sendCommandInput := &ssm.SendCommandInput{
		Comment:      aws.String(command),
		DocumentName: aws.String("AWS-RunShellScript"),
		Parameters:   param,
		InstanceIds:  aws.StringSlice([]string{instanceId}),
	}

	if sendCommandOutput, err := svcSsm.SendCommand(sendCommandInput); err == nil {
		fmt.Printf("Command executed successfully on instance: %v, DNS: %s\n", instanceId, DNSName)
		fmt.Printf("Response: %v\n", sendCommandOutput)
	} else {
		fmt.Printf("Failed to execute command due to %v\n", err)
	}
}

func ShowIPAddress() {
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
					// process IP address
					fmt.Println("IP: ", ip)
				}
			}
		}
	}
}

func BackupHandler(ctx context.Context, snsEvent events.SNSEvent) {
	defer func() { fmt.Println("Terminated.") }()
	NetworkId := getNetworkId()
	t := time.Now()
	fmt.Println("Compiled on ", BuildDate)
	fmt.Printf("Scheduled backup event for %s at %s\n", NetworkId, t.String())
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

	myBucket := getPrintEnv("Bucket")
	myKey := getPrintEnv("Key")
	sshUser := getPrintEnv("SSHUser")

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
		} else {
			sshKey = result.Plaintext
			fmt.Printf("Decryption succeeded, length: %d\n", len(sshKey))
		}
	} else {
		fmt.Printf("Unable to retrieve S3 bucket: %s key: %s due to error: %v\n", myBucket, myKey, err)
		return
	}

	svc := ec2.New(sess)

	filters := []*ec2.Filter{
		// This is the NetworkId that I'm interested in
		&ec2.Filter{
			Name:   aws.String("tag:NetworkId"),
			Values: aws.StringSlice([]string{NetworkId}),
		},
		// These are the roles that are running
		&ec2.Filter{
			Name: aws.String("tag:Role"),
			Values: aws.StringSlice([]string{
				"Maker",
			}),
		},
		// Look at only running instances
		&ec2.Filter{
			Name:   aws.String("instance-state-name"),
			Values: aws.StringSlice([]string{"running"}),
		},
	}
	input := &ec2.DescribeInstancesInput{Filters: filters}

	fmt.Println("Retrieving instances...")
	if resp, err := svc.DescribeInstances(input); err == nil {
		numInst := len(resp.Reservations)
		lucky := rand.Intn(numInst)
		idx := lucky

		fmt.Printf("Number of instances: %d, chose %d\n", numInst, idx+1)

		for _, inst := range resp.Reservations[idx].Instances {
			DNSName := aws.StringValue(inst.PublicDnsName)
			InstanceId := aws.StringValue(inst.InstanceId)
			fmt.Printf("Instance: %s, DNS: %s\n", InstanceId, DNSName)

			sshconfig := softwareupgrade.NewSSHConfigKeyBytes(sshUser, sshKey, DNSName)
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

func DebugEnvironment() (result bool) {
	funcName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	vsCodeLogs := os.Getenv("VSCODE_LOGS")
	result = funcName == "" || vsCodeLogs != "" // is in DebugEnvironment if AWS_LAMBDA_FUNCTION_NAME is empty
	return
}

func main() {
	lambda.Start(BackupHandler)
}
