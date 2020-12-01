// Copyright © 2020 Jonathan Whitaker <jonathan@whitaker.io>

package cmd

import (
	"go/build"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/whitaker-io/machine/cmd/templates"
)

var versionString string
var goVersionString string

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create <path>",
	Short: "Create a new project based on github.com/whitaker-io/machine",
	Long: `Create a new project based on github.com/whitaker-io/machine
	
	Example: machine create github.com/whitaker-io/test
	`,
	Run: func(cmd *cobra.Command, args []string) {
		pathParts := strings.Split(args[0], string(filepath.Separator))

		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			gopath = build.Default.GOPATH
		}

		gosrc := os.Getenv("GOSRC")
		if gosrc == "" {
			gosrc = filepath.Join(gopath, "src")
		}

		dir := filepath.Join(gosrc, args[0])

		settings := map[string]interface{}{
			"Path":      args[0],
			"Name":      pathParts[len(pathParts)-1],
			"Version":   versionString,
			"GoVersion": goVersionString,
		}

		err := templates.GenerateProject(dir, defaultProject, force, settings)
		if err != nil {
			log.Println(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.PersistentFlags().StringVar(
		&versionString,
		"version",
		"0.1.0",
		"(optional, default 0.1.0) alternative version for the generated project",
	)

	createCmd.PersistentFlags().StringVar(
		&goVersionString,
		"go-version",
		"1.15",
		"(optional, default 1.15) alternative version for the generated project",
	)
}

var defaultProject = templates.Project{
	Dirs: map[string]templates.Project{
		".helm": {
			Dirs: map[string]templates.Project{
				"templates": {
					Files: map[string]templates.File{
						"deployment.yaml": {Template: deploymentFile, IgnoreTemplate: true},
						"service.yaml":    {Template: serviceFile, IgnoreTemplate: true},
						"config.yaml":     {Template: helmConfigFile, IgnoreTemplate: true},
						"secrets.yaml":    {Template: helmSecretFile, IgnoreTemplate: true},
					},
				},
			},
			Files: map[string]templates.File{
				"Chart.yaml":  {Template: chartFile},
				"values.yaml": {Template: valuesFile},
			},
		},
		"pipe": {
			Files: map[string]templates.File{
				"pipe.go": {Template: pipeFile},
			},
		},
		"version": {
			Files: map[string]templates.File{
				"version.go": {Template: versionFile},
			},
		},
	},
	Files: map[string]templates.File{
		"main.go":        {Template: mainFile},
		"go.mod":         {Template: modFile},
		"bootstrap.yaml": {Template: configFile},
		"Dockerfile":     {Template: dockerFile},
		".gitignore":     {Template: ignoreFile},
		".dockerignore":  {Template: ignoreFile},
	},
}

const mainFile = `package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"{{.Path}}/pipe"

	homedir "github.com/mitchellh/go-homedir"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfgFile string

func main() {
	initConfig()

	port := viper.GetInt("server.port")
	gracePeriod := viper.GetDuration("server.grace_period")

	if err := pipe.Pipe.Run(context.Background(), ":"+strconv.Itoa(port), gracePeriod*time.Second); err != nil {
		fmt.Printf("error starting pipe - %v\n", err)
	}
}

func init() {
	viper.SetDefault("server.port", 5000)
	viper.SetDefault("server.grace_period", 30 * time.Second)

	home, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	flag.StringVarP(
		&cfgFile, 
		"config", 
		"f", 
		filepath.Join(home, ".{{.Name}}.yaml"), 
		"(optional, default is $HOME/.{{.Name}}.yaml) alternative path to the bootstrap config file",
	)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(filepath.Join(home, ".{{.Name}}.yaml"))
		viper.SetConfigName(".test")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}`

const pipeFile = `package pipe

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/whitaker-io/machine"
)

var (
	// Logger - set logger to enable logging
	Logger machine.Logger

	// LogStore - set logger to enable clustering
	LogStore machine.LogStore

	// Pipe - entrypoint for setting up the streams
	Pipe = machine.NewPipe(uuid.New().String(), Logger, LogStore, fiber.Config{})
)

func boolP(v bool) *bool {
	return &v
}

func intP(v int) *int {
	return &v
}`

const modFile = `module {{.Path}}

go {{.GoVersion}}

require (
	github.com/gofiber/fiber/v2 v2.2.0
	github.com/google/uuid v1.1.2
	github.com/mitchellh/go-homedir v1.1.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
)`

const configFile = `app:
  name: {{.Name}}
	version: {{.Version}}
server:
	port: 5000
	# grace_period is time in seconds to allow the server to shutdown
	grace_period: 30`

const dockerFile = `FROM golang:{{.GoVersion}}-alpine as user-init

ENV CGO_ENABLED="0"
ENV GO111MODULE=on

WORKDIR /go/src/{{.Path}}

RUN apk update && apk upgrade \
 && apk add --update ca-certificates \
 && apk add --update -t deps git gcc libtool build-base curl \
 && addgroup -g 63000 go \
 && adduser -u 63000 -G go -s /bin/sh -D go

COPY . /go/src/{{.Path}}

RUN go build \
				  -a \
					-o="pkg/linux_amd64/{{.Name}}" \
					-ldflags "-s -w"

FROM scratch

COPY --from=user-init /etc/passwd /etc/passwd
COPY --from=user-init /etc/group /etc/group

USER go
ENV HOME /
COPY --chown=go bootstrap.yaml /config/.{{.Name}}.yaml
COPY --chown=go --from=user-init /go/src/{{.Path}}/pkg/linux_amd64/{{.Name}} /

CMD [ "/{{.Name}}"]

ARG BUILD_DATE
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
      org.label-schema.name="{{.Name}}" \
      org.label-schema.description="Provides a Docker image for {{.Name}} built from scratch." \
      org.label-schema.vcs-url="https://{{.Path}}" \
      org.label-schema.version=$VERSION`

const versionFile = `package version

import "fmt"

const Version = "{{.Version}}"

var (
	Name      string
	GitCommit string

	HumanVersion = fmt.Sprintf("%s v%s (%s)", Name, Version, GitCommit)
)`

const ignoreFile = `# Binaries for programs and plugins
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary, built with go test -c
*.test

# Output of the go coverage tool, specifically when used with LiteIDE
*.out

pkg/
vendor/
secret/
volume/
certs/

# General
.DS_Store
.AppleDouble
.LSOverride

# Icon must end with two \r
Icon


# Thumbnails
._*

# Files that might appear in the root of a volume
.DocumentRevisions-V100
.fseventsd
.Spotlight-V100
.TemporaryItems
.Trashes
.VolumeIcon.icns
.com.apple.timemachine.donotpresent

# Directories potentially created on remote AFP share
.AppleDB
.AppleDesktop
Network Trash Folder
Temporary Items
.apdisk

.vscode/*
!.vscode/settings.json
!.vscode/tasks.json
!.vscode/launch.json
!.vscode/extensions.json

# Covers JetBrains IDEs: IntelliJ, RubyMine, PhpStorm, AppCode, PyCharm, CLion, Android Studio and WebStorm
# Reference: https://intellij-support.jetbrains.com/hc/en-us/articles/206544839

# User-specific stuff
.idea/**/workspace.xml
.idea/**/tasks.xml
.idea/**/usage.statistics.xml
.idea/**/dictionaries
.idea/**/shelf

# Generated files
.idea/**/contentModel.xml

# Sensitive or high-churn files
.idea/**/dataSources/
.idea/**/dataSources.ids
.idea/**/dataSources.local.xml
.idea/**/sqlDataSources.xml
.idea/**/dynamic.xml
.idea/**/uiDesigner.xml
.idea/**/dbnavigator.xml

# Gradle
.idea/**/gradle.xml
.idea/**/libraries

# Gradle and Maven with auto-import
# When using Gradle or Maven with auto-import, you should exclude module files,
# since they will be recreated, and may cause churn.  Uncomment if using
# auto-import.
# .idea/modules.xml
# .idea/*.iml
# .idea/modules

# CMake
cmake-build-*/

# Mongo Explorer plugin
.idea/**/mongoSettings.xml

# File-based project format
*.iws

# IntelliJ
out/

# mpeltonen/sbt-idea plugin
.idea_modules/

# JIRA plugin
atlassian-ide-plugin.xml

# Cursive Clojure plugin
.idea/replstate.xml

# Crashlytics plugin (for Android Studio and IntelliJ)
com_crashlytics_export_strings.xml
crashlytics.properties
crashlytics-build.properties
fabric.properties

# Editor-based Rest Client
.idea/httpRequests

# Android studio 3.1+ serialized cache file
.idea/caches/build_file_checksums.ser`

var deploymentFile = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
  labels: {{range $k, $v := .labels}}
    {{$k}}: '{{$v}}' {{end}}
  name: '{{.Name}}'
spec:
  progressDeadlineSeconds: 600
  replicas: {{.deployment.Replicas}}
  revisionHistoryLimit: 10
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: 'RollingUpdate'
  selector:
    matchLabels: {{range $k, $v := .labels}}
      {{$k}}: '{{$v}}' {{end}}
  template:
    metadata:
      labels: {{range $k, $v := .labels}}
        {{$k}}: '{{$v}}' {{end}}
    spec:
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      terminationGracePeriodSeconds: {{.deployment.gracePeriod}}
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 99
            podAffinityTerm:
              labelSelector:
                matchLabels: {{range $k, $v := .labels}}
                  {{$k}}: '{{$v}}' {{end}}
              topologyKey: kubernetes.io/hostname
          - podAffinityTerm:
              labelSelector:
                matchLabels: {{range $k, $v := .labels}}
                  {{$k}}: '{{$v}}' {{end}}
              topologyKey: failure-domain.beta.kubernetes.io/zone
            weight: 100
      containers:
      - name: '{{.Name}}'
        image: '{{.imageRepo}}/{{.name}}:{{.version}}'
        imagePullPolicy: Always 
        env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.hostIP
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        envFrom:
        - configMapRef:
            name: '{{.Name}}-config' {{range $i, $secret := .secrets}}
        - secretRef:
            name: '{{$secret.name}}-secret'{{end}}
        ports:
        - containerPort: {{.deployment.port}}
          name: http
          protocol: TCP
        readinessProbe:
          httpGet:
            path: /health
            port: {{.deployment.port}}
            scheme: HTTP
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 10
          successThreshold: 1
          failureThreshold: 3
        livenessProbe:
          httpGet:
            path: /health
            port: {{.deployment.port}}
            scheme: HTTP
          initialDelaySeconds: 5
          periodSeconds: 30
          timeoutSeconds: 10
          successThreshold: 1
          failureThreshold: 3
        resources:
          limits:
            cpu: '{{.deployment.limits.cpu}}'
            memory: '{{.deployment.limits.Memory}}'
          requests: 
            cpu: '{{.deployment.requests.cpu}}'
            memory: '{{.deployment.requests.Memory}}'
---
apiVersion: autoscaling/v1
kind: HorizontalPodAutoscaler
metadata:
  labels: {{range $k, $v := .labels}}
    {{$k}}: '{{$v}}' {{end}}
  name: '{{.name}}-autoscaler'
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: '{{.name}}'
  minReplicas: {{.deployment.replicas}}
  maxReplicas: {{.deployment.scaleLimit}}
  targetCPUUtilizationPercentage: {{.deployment.scaleTarget}}`

var serviceFile = `{{$root := .}}{{range $i, $service := .services}}
---
apiVersion: v1
kind: Service
metadata:
  labels: {{range $k1, $v1 := $root.labels}}
    {{$k1}}: '{{$v1}}' {{end}}
  name: '{{$service.name}}-svc'
spec:
  externalTrafficPolicy: Cluster
  ports:
  - name: '{{$service.port}}'
    port: {{$service.port}}
    protocol: TCP
    targetPort: {{$service.port}}
  selector: {{range $k1, $v1 := $root.labels}}
    {{$k1}}: '{{$v1}}' {{end}}
  sessionAffinity: None
  type: {{$service.type}} {{end}}`

var helmConfigFile = `apiVersion: v1
data: {{range $k, $v := .config}}
  {{$k | ToUpper}}: '{{$v | Base64}}' {{end}}
kind: ConfigMap
metadata:
	name: {{ .name }}-config`

/* #nosec */
var helmSecretFile = `{{$root := .}}{{range $i, $secret := .secrets}}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ $secret.name }}-secret
type: {{$secret.Type}}
data:
  {{$secret.data | Base64}}{{end}}`

var chartFile = `apiVersion: v2
name: {{.Name}}
version: 0.1.0
description: A Stream Processor Built With The Machine Project https://github.com/whitaker-io/machine
type: Machine Worker`

var valuesFile = `
name: {{.Name}}
version: 0.1.0
deployment:
	port: 5000
	gracePeriod: 10
	replicas: 1
	scaleLimit: 5
	scaleTarget: 75
	limits:
		cpu: 2000m
		memory: 2Gi
	requests:
		cpu: 2000m
		memory: 2Gi
labels:
	name: {{.Name}}
	machine: worker
services:
	- name: {{.Name}}
		port: 5000
config:
	NAME: {{.Name}}
	PORT: 5000
	GRACE_PERIOD: 10
secrets:
	- name: secret-1
		type: opaque
		Data: |
			value`
