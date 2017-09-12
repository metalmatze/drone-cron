package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/drone/drone-go/drone"
	"github.com/ghodss/yaml"
	"github.com/robfig/cron"
	"golang.org/x/oauth2"
)

type ConfigJobs struct {
	Jobs []ConfigJob `json:"jobs"`
}

type ConfigJob struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
}

func main() {
	configPath := os.Getenv("DRONE_CRON_CONFIG")
	host := strings.TrimSuffix(os.Getenv("DRONE_SERVER"), "/")
	token := os.Getenv("DRONE_TOKEN")

	if configPath == "" {
		configPath = "./config.yaml"
	}
	if host == "" {
		log.Fatal("Set DRONE_SERVER which is currently empty")
	}
	if token == "" {
		log.Fatal("Set DRONE_TOKEN which is currently empty")
	}

	// Reading & unmarshalling the config

	config, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	var jobs ConfigJobs
	if err := yaml.Unmarshal(config, &jobs); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Setting up a client to talk to drone with

	httpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	client := drone.NewClient(host, httpClient)

	_, err = client.Self()
	if err != nil {
		log.Fatalf("failed to ping drone: %v\n", err)
	}

	cs := CronScheduler{client: client}

	// Setting up and running Cron

	c := cron.New()

	log.Println("Adding jobs with their schedule:")
	for _, job := range jobs.Jobs {
		log.Println(job.Schedule, job.Name)
		c.AddFunc(job.Schedule, cs.BuildStart(job.Name))
	}

	go c.Run()

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt)

	<-sig
	log.Println("received an interrupt signal, shutting down")
	c.Stop()
	cancel()
}

type CronScheduler struct {
	client drone.Client
}

func (cs *CronScheduler) BuildStart(repo string) cron.FuncJob {
	splits := strings.Split(repo, "/")
	if len(splits) != 2 {
		log.Println("failed to split repo name:", repo)
	}
	owner, name := splits[0], splits[1]

	return func() {
		lastBuild, err := cs.client.BuildLast(owner, name, "master") // TODO: Make branch configurable
		if err != nil {
			log.Printf("failed to get last build: %v\n", err)
			return
		}
		log.Printf("Restarting last build %s/%s#%d", owner, name, lastBuild.Number)

		build, err := cs.client.BuildStart(owner, name, lastBuild.Number, nil)
		if err != nil {
			log.Printf("failed to start new build: %v\n", err)
			return
		}

		log.Printf("Starting build %s/%s#%d\n", owner, name, build.Number)
	}
}
