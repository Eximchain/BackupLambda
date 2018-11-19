package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/kms"
)

func handleErr(err error) {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case kms.ErrCodeNotFoundException, kms.ErrCodeDisabledException,
			kms.ErrCodeKeyUnavailableException, kms.ErrCodeDependencyTimeoutException,
			kms.ErrCodeInvalidKeyUsageException, kms.ErrCodeInvalidGrantTokenException,
			kms.ErrCodeInternalException, kms.ErrCodeInvalidStateException,
			autoscaling.ErrCodeResourceContentionFault, autoscaling.ErrCodeServiceLinkedRoleFailure,
			autoscaling.ErrCodeResourceInUseFault:
			fmt.Printf("Code: %s, error: %s\n", aerr.Code(), aerr.Error())
		default:
			fmt.Println(aerr.Error())
		}
	} else {
		fmt.Println(err.Error())
	}
}
