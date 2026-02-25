package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"ajiasu-proxy-api/internal/ajiasu"
	"ajiasu-proxy-api/internal/api"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Ajiasu AjiasuConfig `yaml:"ajiasu"`
}

type ServerConfig struct {
	Port  int    `yaml:"port"`
	Token string `yaml:"token"`
}

type AjiasuConfig struct {
	Binary string `yaml:"binary"`
}

func main() {
	cfg := loadConfig()

	mgr := ajiasu.New(cfg.Ajiasu.Binary)
	if err := mgr.Login(); err != nil {
		log.Printf("爱加速登录失败: %v", err)
	} else {
		log.Println("爱加速登录成功")
	}

	router := api.NewRouter(cfg.Server.Token, mgr)
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("API 服务启动在 %s", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func loadConfig() Config {
	cfg := Config{
		Server: ServerConfig{Port: 8080},
	}

	configPath := "configs/app.yaml"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}
	return cfg
}
