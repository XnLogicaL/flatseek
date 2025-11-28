package flatseek

import (
	"bufio"
	"os"
	"os/exec"
	"os/signal"
)

// installs or removes a package
func (ps *UI) installPackage(pkg Package, installed bool) {
	exec.Command("echo", "install works!!!1!")
	exec.Command("sleep", "10")
}

// installs or removes a package
func (ps *UI) installSelectedPackage() {
	if ps.selectedPackage == nil {
		return
	}
	row, _ := ps.tablePackages.GetSelection()
	installed := ps.tablePackages.GetCell(row, 2).Reference == true

	ps.installPackage(*ps.selectedPackage, installed)
}

// issues "Update command"
func (ps *UI) performUpgrade(aur bool) {
	command := ps.conf.SysUpgradeCommand
	if aur && ps.conf.AurUseDifferentCommands && ps.conf.AurUpgradeCommand != "" {
		command = ps.conf.AurUpgradeCommand
	}

	args := []string{"-c", command}

	ps.runCommand(ps.shell, args...)
}

// suspends UI and runs a command in the terminal
func (ps *UI) runCommand(command string, args ...string) {
	// suspend gui and run command in terminal
	ps.app.Suspend(func() {

		cmd := exec.Command(command, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// handle SIGINT and forward to the child process
		cmd.Start()
		quit := handleSigint(cmd)
		err := cmd.Wait()
		if err != nil {
			if err.Error() != "signal: interrupt" {
				cmd.Stdout.Write([]byte("\n" + err.Error() + "\nPress ENTER to return to flatseek\n"))
				r := bufio.NewReader(cmd.Stdin)
				r.ReadLine()
			}
		}
		quit <- true
	})
}

// handles SIGINT call and passes it to a cmd process
func handleSigint(cmd *exec.Cmd) chan bool {
	quit := make(chan bool, 1)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		select {
		case <-c:
			if cmd != nil {
				cmd.Process.Signal(os.Interrupt)
			}
		case <-quit:
		}
	}()
	return quit
}
