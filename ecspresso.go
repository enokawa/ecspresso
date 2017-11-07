package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/kayac/go-config"
	"github.com/pkg/errors"
)

type TaskDefinitionContainer struct {
	TaskDefinition TaskDefinition `yaml:"taskDefinition" json:"taskDefinition"`
}

type TaskDefinition struct {
	ContainerDefinitions []map[string]interface{} `yaml:"containerDefinitions" json:"containerDefinitions"`
	Family               string                   `yaml:"family" json:"family"`
	NetworkMode          string                   `yaml:"networkMode" json:"networkMode"`
	PlacementConstraints []map[string]string      `yaml:"placementConstraints" json:"placementConstraints"`
	RequiresAttributes   []map[string]string      `yaml:"requiresAttributes" json:"requiresAttributes"`
	Revision             int                      `yaml:"revision" json:"revision"`
	Status               string                   `yaml:"status" json:"status"`
	TaskRoleArn          string                   `yaml:"taskRoleArn" json:"taskRoleArn"`
	Volumes              []map[string]interface{} `yaml:"volumes" yaml:"json"`
}

func (t *TaskDefinition) Name() string {
	return fmt.Sprintf("%s:%d", t.Family, t.Revision)
}

type App struct {
	Service        string
	Cluster        string
	TaskDefinition *TaskDefinition
	Registered     *TaskDefinition
}

func (d *App) DescribeServiceDeployments(ctx context.Context) error {
	b, err := awsECS(ctx, "describe-services",
		"--service", d.Service,
		"--cluster", d.Cluster,
	)
	if err != nil {
		d.Log(string(b))
		return err
	}
	var sc ServiceContainer
	if err := json.Unmarshal(b, &sc); err != nil {
		return err
	}
	if len(sc.Services) > 0 {
		for _, dep := range sc.Services[0].Deployments {
			d.Log(dep.String())
		}
	}
	return nil
}

func Run(conf *Config) error {
	var cancel context.CancelFunc
	ctx := context.Background()
	if conf.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, conf.Timeout)
		defer cancel()
	}

	d := &App{
		Service: conf.Service,
		Cluster: conf.Cluster,
	}
	d.Log("Starting ecspresso")

	if err := d.DescribeServiceDeployments(ctx); err != nil {
		return err
	}
	if err := d.LoadTaskDefinition(conf.TaskDefinitionPath); err != nil {
		return err
	}
	if err := d.RegisterTaskDefinition(ctx); err != nil {
		return err
	}
	if err := d.UpdateService(ctx); err != nil {
		return err
	}
	if err := d.WaitServiceStable(ctx); err != nil {
		return err
	}

	d.Log("Service is stable now. Completed!")
	return nil
}

func (d *App) Name() string {
	return fmt.Sprintf("%s/%s", d.Service, d.Cluster)
}

func (d *App) Log(v ...interface{}) {
	args := []interface{}{d.Name()}
	args = append(args, v...)
	log.Println(args...)
}

func (d *App) WaitServiceStable(ctx context.Context) error {
	d.Log("Waiting for service stable...(it will take a few minutes)")

	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		tick := time.Tick(10 * time.Second)
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick:
				d.DescribeServiceDeployments(waitCtx)
			}
		}
	}()

	_, err := awsECS(ctx, "wait", "services-stable",
		"--service", d.Service,
		"--cluster", d.Cluster,
	)
	return err
}

func (d *App) UpdateService(ctx context.Context) error {
	d.Log("Updating service...")
	_, err := awsECS(ctx, "update-service",
		"--service", d.Service,
		"--cluster", d.Cluster,
		"--task-definition", d.Registered.Name(),
	)
	return err
}

func (d *App) RegisterTaskDefinition(ctx context.Context) error {
	d.Log("Registering a new task definition...")

	b, err := awsECS(ctx, "register-task-definition",
		"--output", "json",
		"--family", d.TaskDefinition.Family,
		"--task-role-arn", d.TaskDefinition.TaskRoleArn,
		"--network-mode", d.TaskDefinition.NetworkMode,
		"--volumes", toJSON(d.TaskDefinition.Volumes),
		"--placement-constraints", toJSON(d.TaskDefinition.PlacementConstraints),
		"--container-definitions", toJSON(d.TaskDefinition.ContainerDefinitions),
	)
	if err != nil {
		return err
	}
	var res TaskDefinitionContainer
	if err := json.Unmarshal(b, &res); err != nil {
		return errors.Wrap(err, "register-task-definition parse response failed")
	}
	d.Log("Task definition is registered", res.TaskDefinition.Name())
	d.Registered = &res.TaskDefinition
	return nil
}

func (d *App) LoadTaskDefinition(path string) error {
	d.Log("Creating a new task definition by", path)
	var c TaskDefinitionContainer
	if err := config.LoadWithEnvJSON(&c, path); err != nil {
		return err
	}
	d.TaskDefinition = &c.TaskDefinition
	return nil
}

func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func awsECS(ctx context.Context, subCommand string, args ...string) ([]byte, error) {
	_args := []string{"ecs", subCommand}
	_args = append(_args, args...)
	cmd := exec.CommandContext(ctx, "aws", _args...)
	b, err := cmd.Output()
	if err != nil {
		if _e, ok := err.(*exec.ExitError); ok {
			fmt.Fprintln(os.Stderr, string(_e.Stderr))
		}
		return nil, errors.Wrap(err, subCommand+" failed")
	}
	return b, nil
}