# How to build the Backup Lambda

This repo contains the source to the Backup Lambda.

In order to build this, set the BUILD_DATE, GOOS, GOARCH and GOPATH variables, then run the go build command, for example:

BUILD_DATE=`date`
GOOS="linux"
GOARCH="amd64"
GOPATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/" 

go build -ldflags "-X 'main.BuildDate=$BUILD_DATE'" -v eximchain/BackupLambda
