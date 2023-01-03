package controllers

import "time"

type SloopControllerConfig struct {
	Name   string            `json:"name"`
	Config Config            `json:"config"`
	Status SloopConfigStatus `json:"status"`
}
type TemplateFile struct {
	FileName     string `json:"fileName"`
	ManifestYaml string `json:"manifest_yaml"`
}
type Component struct {
	Name          string         `json:"name"`
	Namespace     string         `json:"namespace"`
	Path          string         `json:"path"`
	TemplateFiles []TemplateFile `json:"templateFiles"`
}
type Config struct {
	Components           []Component `json:"components"`
	ConsolidatedManifest string      `json:"consolidated_manifest"`
}

//SloopConfigStatus - The overall status of the sloop config sync action
type SloopConfigStatus struct {
	DeployedOn   time.Time `json:"deployed_on" yaml:"updated"`
	SyncRevision int       `json:"sync_revision" yaml:"revision"`
	Version      string    `json:"version" yaml:"version"`
	Components   string    `json:"components" yaml:"components"`
	Name         string    `json:"name" yaml:"package"`
}
