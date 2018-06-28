package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/namely/k8s-pipeliner/pipeline"
	"github.com/namely/k8s-pipeliner/pipeline/builder"
	"github.com/namely/k8s-pipeliner/pipeline/config"
	"github.com/urfave/cli"
)

var (
	// Version defines the current version of k8s-pipeliner
	version = "n/a"
)

func main() {
	app := cli.NewApp()
	app.Name = "k8s-pipeliner"
	app.Description = "create spinnaker pipelines from kubernetes clusters"
	app.Flags = []cli.Flag{}
	app.Version = version

	app.Commands = []cli.Command{
		{
			Name:   "create",
			Usage:  "creates a spinnaker pipeline for a given application on multiple k8s clusters",
			Action: createAction,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "linear, l",
					Usage: "Assigns refs and reliesOn identifiers for you so you dont need to specify them. This is useful if your pipelines are always linear.",
				},
				cli.BoolFlag{
					Name:  "v2",
					Usage: "Create your manifests with the v2 kubernetes provider",
				},
			},
		},
		{
			Name:   "validate",
			Usage:  "performs simple validation on a pipeline to ensure it will work with Spinnaker + Kubernetes",
			Action: validateAction,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func createAction(ctx *cli.Context) error {
	pipelineFile := ctx.Args().First()
	if pipelineFile == "" {
		return errors.New("missing parameter: file")
	}

	f, err := os.Open(pipelineFile)
	if err != nil {
		return err
	}

	p, err := config.NewPipeline(f)
	if err != nil {
		return err
	}

	builder := builder.New(p, builder.WithV2Provider(ctx.Bool("v2")), builder.WithLinear(ctx.Bool("linear")))
	return json.NewEncoder(os.Stdout).Encode(builder)
}

func validateAction(ctx *cli.Context) error {
	p, err := pipelineConfigHelper(ctx)
	if err != nil {
		return err
	}

	return pipeline.NewValidator(p).Validate()
}

func pipelineConfigHelper(ctx *cli.Context) (*config.Pipeline, error) {
	pipelineFile := ctx.Args().First()
	if pipelineFile == "" {
		return nil, errors.New("missing parameter: file")
	}

	f, err := os.Open(pipelineFile)
	if err != nil {
		return nil, err
	}

	p, err := config.NewPipeline(f)
	if err != nil {
		return nil, err
	}

	return p, nil
}
