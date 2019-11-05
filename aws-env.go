package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"log"
	"os"
	"strings"
)

const (
	formatExports = "exports"
	formatDotenv  = "dotenv"
	formatJavaProp  = "prop"
)

func main() {
	if os.Getenv("AWS_ENV_PATH") == "" {
		log.Println("aws-env running locally, without AWS_ENV_PATH")
		return
	}

	recursivePtr := flag.Bool("recursive", false, "recursively process parameters on path")
	format := flag.String("format", formatExports, "output format")
	flag.Parse()

	if *format != formatExports && *format != formatDotenv && *format != formatJavaProp{
		log.Fatal("Unsupported format option. Must be 'exports', 'dotenv', or 'prop'")
		os.Exit(1)
	}

	sess := CreateSession()
	client := CreateClient(sess)

	ExportVariables(client, os.Getenv("AWS_ENV_PATH"), *recursivePtr, *format, "")
}

func CreateSession() *session.Session {
	return session.Must(session.NewSession())
}

func CreateClient(sess *session.Session) *ssm.SSM {
	return ssm.New(sess)
}

func ExportVariables(client *ssm.SSM, path string, recursive bool, format string, nextToken string) {
	input := &ssm.GetParametersByPathInput{
		Path:           &path,
		WithDecryption: aws.Bool(true),
		Recursive:      aws.Bool(recursive),
	}

	if nextToken != "" {
		input.SetNextToken(nextToken)
	}

	output, err := client.GetParametersByPath(input)

	if err != nil {
		log.Panic(err)
	}

	for _, element := range output.Parameters {
		OutputParameter(path, element, getLatestDescription(client, element), format)
	}

	if output.NextToken != nil {
		ExportVariables(client, path, recursive, format, *output.NextToken)
	}
}

func getLatestDescription(client *ssm.SSM, parameter *ssm.Parameter) string {
	description := ""

	input := &ssm.GetParameterHistoryInput{
		Name: parameter.Name,
		//MaxResults: func(i int64) *int64 { return &i }(1),
	}

	for {
		output, err := client.GetParameterHistory(input)

		if err != nil {
			log.Panic(err)
		}

		for _, history := range output.Parameters {
			if history.Description != nil {
				description = *history.Description
			}
		}

		if output.NextToken == nil {
			break
		}

		input.NextToken = output.NextToken
	}

	return description
}

func OutputParameter(path string, parameter *ssm.Parameter, description string, format string) {
	name := *parameter.Name
	value := *parameter.Value

	env := strings.Replace(strings.Trim(name[len(path):], "/"), "/", "_", -1)
	value = strings.Replace(value, "\n", "\\n", -1)

	switch format {
	case formatExports:
		fmt.Printf("export %s=$'%s'\n", env, value)
	case formatDotenv:
		fmt.Printf("%s=\"%s\"\n", env, value)
	case formatJavaProp:
		if description != "" {
			fmt.Printf("# %s\n", description)
		}
		key := strings.Replace(strings.Trim(name[len(path):], "/"), "/", ".", -1)
		fmt.Printf("%s = %s\n", key, value)
	}
}
