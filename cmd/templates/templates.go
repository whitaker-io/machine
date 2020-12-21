package templates

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/google/uuid"
)

// TemplatesConfigKey -- config.templates -- []models.Config
const TemplatesConfigKey = "config.templates"

var registry = map[string]string{}

var funcMap = template.FuncMap{
	"ToLower": strings.ToLower,
	"UUID": func() string {
		return uuid.New().String()
	},
}

// RegisterFNMap adds the given template.FuncMap to the registry by name
func RegisterFNMap(key string, i interface{}) {
	funcMap[key] = i
}

// Project struct for holding project hierarchy
type Project struct {
	Dirs  map[string]Project
	Files map[string]File
}

// File struct for holding template info
type File struct {
	Template       string
	IgnoreTemplate bool
}

// GenerateProject function for creating a project
func GenerateProject(path string, project Project, force bool, settings map[string]interface{}) error {
	for dir, p := range project.Dirs {
		dirPath := filepath.Join(path, dir)
		makeDir(dirPath)
		err := GenerateProject(dirPath, p, force, settings)

		if err != nil {
			return err
		}
	}

	for file, templateKey := range project.Files {
		payload, err := templateKey.Template, error(nil)

		if !templateKey.IgnoreTemplate {
			payload, err = GenerateFile(file, templateKey.Template, settings)
		}

		if err != nil {
			return err
		}

		if payload2, err := ioutil.ReadFile(filepath.Join(path, file)); !force && err == nil && payload != string(payload2) {
			fmt.Printf("Conflict file: %s -- not forcing\n", filepath.Join(path, file))
		} else if err := ioutil.WriteFile(filepath.Join(path, file), []byte(payload), 0600); err != nil {
			return err
		}
	}

	return nil
}

// GenerateFile function to take a template and fill it in
func GenerateFile(name, templatePayload string, settings map[string]interface{}) (string, error) {
	t := template.Must(template.New(name).Funcs(funcMap).Parse(templatePayload))

	bb := &bytes.Buffer{}
	err := t.Execute(bb, settings)

	if err != nil {
		return "", fmt.Errorf("error executing template %s - s%v", name, err)
	}

	return bb.String(), nil
}

// RegisterTemplate func to add a template to the registry
func RegisterTemplate(name, templatePayload string) {
	registry[name] = templatePayload
}

func makeDir(name string) {
	if _, err := os.Stat(name); os.IsExist(err) {
		return
	}

	err := os.MkdirAll(name, os.ModePerm)
	if err != nil {
		panic(err)
	}
}
