package main

import (
	"os/exec"
	"strings"
	"github.com/mattn/go-pipeline"
	"os"
	"io"
	"bytes"
	"sync"
	"fmt"
)

type Params struct {
	args   []string
	stdOut io.Writer
	stdErr io.Writer
}

func main() {
	args := make([]string, len(os.Args) - 1)
	copy(args, os.Args[1:])

	params := Params{}
	params.args = args

	// no args or device indicated
	if len(args) == 0 || args[0] == "-s"{
		execAdb(params)
		return
	}

	// check if calling command unessential giving device serial
	serialNotRequiredCommands := map[string]int{"help": 0,	"devices": 1,	"version": 2,	"start-server": 3,	"kill-server": 4}
	_, ok := serialNotRequiredCommands[args[0]]
	if ok {
		execAdb(params)
		return
	}

	devices := createDevicesMap()

	switch len(devices) {
	case 0:
		fmt.Println("no device may be connected")
	case 1:
		execAdb(params)
	default:
		selectedSerial, err := selectDevice(devices)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		deviceSelectingArgs := []string{"-s", selectedSerial}
		deviceSelectingArgs = append(deviceSelectingArgs, args...)
		params.args = deviceSelectingArgs
		execAdb(params)
	}
}

// execute adb
func execAdb(params Params) error {
	c := exec.Command("adb", params.args...)
	if params.stdOut != nil {
		c.Stdout = params.stdOut
	} else {
		c.Stdout = os.Stdout
	}
	if params.stdErr != nil {
		c.Stderr = params.stdErr
	} else {
		c.Stderr = os.Stderr
	}
	c.Stdin = os.Stdin
	return c.Run()
}

// select device according to passed map
func selectDevice(devices map[string]string) (string, error) {
	names := ""
	for name := range devices {
		names += name + "\n"
	}

	out, err := pipeline.Output(
		[]string{"printf", names},
		[]string{"peco"},
	)

	if err != nil {
		return "", err
	}

	selected := strings.TrimRight(string(out), "\n")
	v, ok := devices[selected]

	if ok {
		return v, nil
	} else {
		return "", fmt.Errorf("there is no device: %s", selected)
	}
}

// create device name to serial map
func createDevicesMap() map[string]string {

	devices := map[string]string{}

	serials, err := listDeviceSerials()
	if err != nil {
		return devices
	}

	wg := new(sync.WaitGroup)
	mutex := new(sync.Mutex)
	for _, serial := range serials {
		wg.Add(1)
		go func(serial string, mutex *sync.Mutex, waitGroup *sync.WaitGroup) {
			stdOut := &bytes.Buffer{}
			params := Params{args: []string{"-s", serial, "shell", "cat", "/system/build.prop"}, stdOut:stdOut}
			err := execAdb(params)
			if err != nil {
				wg.Done()
				return
			}
			mutex.Lock()
			defer mutex.Unlock()
			for _, line := range strings.Split(stdOut.String(), "\n") {
				if strings.HasPrefix(line, "ro.product.model=") {
					name := strings.TrimSpace(strings.Split(line, "=")[1])
					devices[name] = serial
					break
				}
			}
			wg.Done()
		}(serial, mutex, wg)
	}
	wg.Wait()

	return devices
}

// get device serials from "adb devices"
func listDeviceSerials() ([]string, error) {
	stdOut := &bytes.Buffer{}
	params := Params{args: []string{"devices"}, stdOut:stdOut}
	err := execAdb(params)
	if err != nil {
		return nil, err
	}
	output := strings.Split(stdOut.String(), "\n")
	deviceSerials := output[:0]
	for _, line := range output {
		if line != "" && !strings.HasPrefix(line, "* daemon") && line != "List of devices attached" {
			deviceSerials = append(deviceSerials, strings.Split(line, "\t")[0])
		}
	}
	return deviceSerials, nil
}