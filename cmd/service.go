package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/service"
	ksvc "github.com/kardianos/service"
)

type Program struct {
	monitoringService *service.MonitoringService
}

func (p *Program) Start(s ksvc.Service) error {
	go p.run()
	return nil
}

func (p *Program) run() {
	p.monitoringService.Run()
}

func (p *Program) Stop(s ksvc.Service) error {
	p.monitoringService.Shutdown()
	return nil
}

func RunService() {
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	if err := logger.InitLogger(cfg); err != nil {
		fmt.Printf("Error initializing logger: %v\n", err)
		return
	}

	monitoringService := service.NewMonitoringService(
		filepath.Join(config.GetConfigDir(), "monitoring_state.json"),
		logger.Logger,
	)

	prg := &Program{
		monitoringService: monitoringService,
	}

	svcConfig := &ksvc.Config{
		Name:        "FanslyScraper",
		DisplayName: "Fansly Scraper Service",
		Description: "This service monitors and records Fansly streams.",
	}

	s, err := ksvc.New(prg, svcConfig)
	if err != nil {
		logger.Logger.Printf("Error creating service: %v", err)
		return
	}

	err = s.Run()
	if err != nil {
		logger.Logger.Printf("Error running service: %v", err)
	}
}
