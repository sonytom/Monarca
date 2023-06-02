package main

import "strings"

type Config struct {
	Url            string `config:"MONARCA_CRUD_URL" default:"https://harmandesenv.simfrete.com/scms/script/"`
	Port           string `config:"MONARCA_PORT" default:":8902"`
	PreSharedToken string `config:"MONARCA_CRUD_TOKEN" default:"5fN7bDnkJVBzPwttCtn06VmZUYdTreBo3FJN49IXi2XaixOJlA"`
	FilePath       string `config:"MONARCA_FILEPATH" default:"D:/Empresa/Tomas/SimuladorFrete/src/java/atualizaDatabase/"`
	Extension      string `config:"MONARCA_EXTENSION" default:".xml"`
}

var config Config

func (same *Config) MakePort() string {
	if !strings.HasPrefix(same.Port, ":") {
		return ":" + same.Port
	}
	return same.Port
}
