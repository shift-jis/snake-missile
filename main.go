package main

import (
	"errors"
	"log"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/shift-jis/snake-missile/application"
)

func main() {
	var arguments application.ProgramArguments
	flagParser := flags.NewParser(&arguments, flags.Default)
	if _, err := flagParser.Parse(); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && errors.Is(flagsErr.Type, flags.ErrHelp) {
			os.Exit(0)
		}
		os.Exit(1)
	}

	properties, err := arguments.LoadProperties()
	if err != nil {
		log.Fatalf("Failed to load properties: %v", err)
	}

	missileManager := application.NewMissileManager(properties)
	if err = missileManager.InitializeEarthworms(); err != nil {
		log.Fatalf("Failed to initialize earthworms: %v", err)
		return
	}

	missileManager.InitializeListeners()
	missileManager.StartConnections()
	missileManager.ManageConnections()
	missileManager.WaitGroup.Wait()
}
