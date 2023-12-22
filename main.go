package main

import (
	"flag"
	"fmt"
	"github.com/kardianos/service"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var domain = flag.String("d", "eber.vip", "Domain(eg: eber.vip)")

var every = flag.Int("t", 300, "Interval time/second(eg: 300)")

var command = flag.String("s", "", "Service management, support install, uninstall, restart")

type program struct{}

var newIp = new(string)

var oldIp = new(string)

func main() {
	// 检查是否有ufw
	ufw := exec.Command("ufw", "version")
	_, err := ufw.CombinedOutput()
	if err != nil {
		log.Println("UFW is not installed, please install it first")
		return
	}
	flag.Parse()
	switch *command {
	case "install":
		installService()
	case "uninstall":
		uninstallService()
	case "restart":
		restartService()
	default:
		s := getService()
		status, _ := s.Status()
		if status != service.StatusUnknown {
			s.Run()
		} else {
			log.Println("The installation service can be run using ./dufw -s install")
			run()
		}
	}
}

func run() {
	for {
		RunOnce()
		time.Sleep(time.Duration(*every) * time.Second)
	}
}

func RunOnce() {
	addr, err := net.ResolveIPAddr("ip4", *domain)
	if err != nil {
		log.Println("Domain name resolution failed:", err)
	}
	// 获取当前IP
	*newIp = addr.IP.String()
	log.Println("Last IP: ", *oldIp)
	log.Println("Current IP: ", *newIp)
	// 判断IP是否变化
	if *oldIp == *newIp {
		log.Println("IP has not changed, no action required")
		return
	}
	// 先判断是否在规则中
	cmd := exec.Command("ufw", "status", "numbered")
	combinedOutput, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("执行命令失败:", err)
	}
	resp := string(combinedOutput)
	containsNew := strings.Contains(resp, *newIp)
	containsOld := strings.Contains(resp, *oldIp)
	if containsNew {
		// 有相同IP则直接退出本次操作
		*oldIp = *newIp
		log.Println("There are already the same rules")
		return
	}
	// 添加新的规则
	cmd = exec.Command("ufw", "allow", "from", *newIp)
	if err := cmd.Run(); err != nil {
		log.Println("Failed to add new rule: ", err)
		return
	}
	log.Printf("New IP: %s added successfully \n", *newIp)
	if *oldIp != "" && containsOld {
		// 删除指定的规则
		cmd = exec.Command("ufw", "delete", "allow", "from", *oldIp)
		if err := cmd.Run(); err != nil {
			log.Println("Delete old rule failed: ", err)
			return
		}
		log.Printf("Old IP: %s deleted successfully \n", *oldIp)
	}
	*oldIp = *newIp
}

func installService() {
	s := getService()
	status, err := s.Status()
	if err != nil && status == service.StatusUnknown {
		if err = s.Install(); err == nil {
			s.Start()
			log.Println("Install dufw successfully!")
			if service.ChosenSystem().String() == "unix-systemv" {
				if _, err := exec.Command("/etc/init.d/dufw", "enable").Output(); err != nil {
					log.Println(err)
				}
				if _, err := exec.Command("/etc/init.d/dufw", "start").Output(); err != nil {
					log.Println(err)
				}
			}
			return
		}
		log.Printf("Install dufw failed: %s \n", err)
	}
	if status != service.StatusUnknown {
		log.Println("dufw is already installed, no need to install again")
	}
}

// uninstallService 卸载服务
func uninstallService() {
	s := getService()
	s.Stop()
	if service.ChosenSystem().String() == "unix-systemv" {
		if _, err := exec.Command("/etc/init.d/dufw", "stop").Output(); err != nil {
			log.Println(err)
		}
	}
	if err := s.Uninstall(); err == nil {
		log.Println("dufw uninstall successfully!")
	} else {
		log.Printf("dufw uninstall failed : %s \n", err)
	}
}

// restartService 重启服务
func restartService() {
	s := getService()
	status, err := s.Status()
	if err == nil {
		if status == service.StatusRunning {
			if err = s.Restart(); err == nil {
				log.Println("Restart dufw successfully!")
			}
		} else if status == service.StatusStopped {
			if err = s.Start(); err == nil {
				log.Println("Start dufw successfully!")
			}
		}
	} else {
		log.Println("dufw service is not installed, please install it first!")
	}
}

const SysvScript = `#!/bin/sh /etc/rc.common
DESCRIPTION="{{.Description}}"
cmd="{{.Path}}{{range .Arguments}} {{.|cmd}}{{end}}"
name="{{.Name}}"
pid_file="/var/run/$name.pid"
stdout_log="/var/log/$name.log"
stderr_log="/var/log/$name.err"
START=99
get_pid() {
    cat "$pid_file"
}
is_running() {
    [ -f "$pid_file" ] && cat /proc/$(get_pid)/stat > /dev/null 2>&1
}
start() {
	if is_running; then
		echo "Already started"
	else
		echo "Starting $name"
		{{if .WorkingDirectory}}cd '{{.WorkingDirectory}}'{{end}}
		$cmd >> "$stdout_log" 2>> "$stderr_log" &
		echo $! > "$pid_file"
		if ! is_running; then
			echo "Unable to start, see $stdout_log and $stderr_log"
			exit 1
		fi
	fi
}
stop() {
	if is_running; then
		echo -n "Stopping $name.."
		kill $(get_pid)
		for i in $(seq 1 10)
		do
			if ! is_running; then
				break
			fi
			echo -n "."
			sleep 1
		done
		echo
		if is_running; then
			echo "Not stopped; may still be shutting down or shutdown may have failed"
			exit 1
		else
			echo "Stopped"
			if [ -f "$pid_file" ]; then
				rm "$pid_file"
			fi
		fi
	else
		echo "Not running"
	fi
}
restart() {
	stop
	if is_running; then
		echo "Unable to stop, will not attempt to start"
		exit 1
	fi
	start
}
`

func (p *program) Start(s service.Service) error {
	go p.run()
	return nil
}
func (p *program) run() {
	run()
}
func (p *program) Stop(s service.Service) error {
	return nil
}

func getService() service.Service {
	options := make(service.KeyValue)
	var depends []string
	options["SysvScript"] = SysvScript
	// 添加网络依赖
	depends = append(depends, "Requires=network.target", "After=network-online.target")
	svcConfig := &service.Config{
		Name:         "dufw",
		DisplayName:  "dufw",
		Description:  "regularly update UFW rules based on the specified DDNS domain name",
		Arguments:    []string{strconv.Itoa(*every), "-d", *domain},
		Dependencies: depends,
		Option:       options,
	}
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalln(err)
	}
	return s
}
