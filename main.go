package main

import (
	"fmt"
	"log"
	"os"

	"github.com/XnLogicaL/flatseek/internal/args"
	"github.com/XnLogicaL/flatseek/internal/config"
	"github.com/XnLogicaL/flatseek/internal/flatseek"
)

const helpText = `
Usage: flatseek [OPTION] [SEARCH-TERM]
	-s	Search-term
	-a	ASCII mode
	-m	Monochrome mode
	-u	show upgrades after startup
	-i	show installed packages after startup

`

func main() {
	logFile, _ := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	log.SetOutput(logFile)

	if os.Getuid() == 0 {
		fmt.Println("flatseek should not be run as root.")
		os.Exit(1)
	}

	f := args.Parse()
	if f.Help {
		printHelp()
		os.Exit(0)
	}

	conf, err := config.Load()
	if err != nil {
		if os.IsNotExist(err) && conf != nil {
			err = conf.Save()
			if err != nil {
				printErrorExit("Error saving configuration file", err)
			}
		} else {
			printErrorExit("Error loading configuration file", err)
		}
	}
	ps, err := flatseek.New(conf, f)
	if err != nil {
		printErrorExit("Error during flatseek initialization", err)
	}
	if err = ps.Start(); err != nil {
		printErrorExit("Error starting flatseek", err)
	}
}

func printErrorExit(message string, err error) {
	fmt.Printf("%s:\n\n%s\n", message, err.Error())
	os.Exit(1)
}

func printHelp() {
	fmt.Printf("%s", helpText)
}
