package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/jbrekelmans/go-winrm"
)

const defaultParallelism = 1

func main() {
	hostFlag := flag.String("host", "", "")
	portFlag := flag.String("port", "5986", "")
	userFlag := flag.String("user", "", "")
	passwordFlag := flag.String("password", "", "")
	parallelismFlag := flag.String("parallelism", strconv.Itoa(defaultParallelism), "")
	flag.Parse()
	port, err := strconv.Atoi(*portFlag)
	if err != nil {
		fmt.Printf("error parsing port: %v\n", err)
	}
	shellCount, err := strconv.Atoi(*parallelismFlag)
	if err != nil {
		fmt.Printf("error parsing parallelism: %v\n", err)
	}
	if shellCount < 1 {
		shellCount = 1
	}
	useTLS := true
	maxEnvelopeSize := 500 * 1000
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: 300,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	c, err := winrm.NewClient(context.Background(), useTLS, *hostFlag, port, *userFlag, *passwordFlag, httpClient, &maxEnvelopeSize)
	if err != nil {
		fmt.Printf("error while initializing winrm client: %v\n", err)
	}
	shells := make([]*winrm.Shell, shellCount)
	for i := 0; i < shellCount; i++ {
		var err1 error
		shells[i], err1 = c.CreateShell()
		if err1 != nil {
			for j := i; j > 0; j-- {
				err2 := shells[j].Close()
				if err2 != nil {
					fmt.Printf("error while closing shell: %w", err2)
				}
			}
			fmt.Printf("error while creating remote shell: %w", err1)
		}
	}
	defer func() {
		for _, shell := range shells {
			err := shell.Close()
			if err != nil {
				fmt.Printf("error while closing shell: %w\n", err)
			}
		}
	}()
	localRoot := "."
	remoteRoot := "C:\\workspace"
	winrm.MustRunCommand(shells[0], `winrm get winrm/config`, nil, true, false)
	winrm.MustRunCommand(shells[0], fmt.Sprintf(`if exist "%s" rd /s /q "%s"`, remoteRoot+"\\", remoteRoot), nil, true, false)
	copier, err := winrm.NewFileTreeCopier(shells, remoteRoot, localRoot)
	if err != nil {
		log.Fatalf("error creating copier: %v", err)
	}
	err = copier.Run()
	if err != nil {
		log.Fatalf("error while copying file tree: %v", err)
	}
	winrm.MustRunCommand(shells[0], fmt.Sprintf(`dir "%s"`, remoteRoot), nil, true, false)
	winrm.MustRunCommand(shells[0], fmt.Sprintf(`type "%s"`, remoteRoot+"\\README.md"), nil, true, false)
}
