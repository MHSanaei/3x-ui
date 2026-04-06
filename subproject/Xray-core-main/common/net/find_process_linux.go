//go:build linux

package net

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/xtls/xray-core/common/errors"
)

func FindProcess(dest Destination) (PID int, Name string, AbsolutePath string, err error) {
	isLocal, err := IsLocal(dest.Address.IP())
	if err != nil {
		return 0, "", "", errors.New("failed to determine if address is local: ", err)
	}
	if !isLocal {
		return 0, "", "", ErrNotLocal
	}
	if dest.Network != Network_TCP && dest.Network != Network_UDP {
		panic("Unsupported network type for process lookup.")
	}
	// the core should never has a domain as source(?
	if dest.Address.Family() == AddressFamilyDomain {
		panic("Domain addresses are not supported for process lookup.")
	}
	var procFile string

	switch dest.Network {
	case Network_TCP:
		if dest.Address.Family() == AddressFamilyIPv4 {
			procFile = "/proc/net/tcp"
		}
		if dest.Address.Family() == AddressFamilyIPv6 {
			procFile = "/proc/net/tcp6"
		}
	case Network_UDP:
		if dest.Address.Family() == AddressFamilyIPv4 {
			procFile = "/proc/net/udp"
		}
		if dest.Address.Family() == AddressFamilyIPv6 {
			procFile = "/proc/net/udp6"
		}
	default:
		panic("Unsupported network type for process lookup.")
	}

	targetHexAddr, err := formatLittleEndianString(dest.Address, dest.Port)
	if err != nil {
		return 0, "", "", errors.New("failed to format address: ", err)
	}

	inode, err := findInodeInFile(procFile, targetHexAddr)
	if err != nil {
		return 0, "", "", errors.New("could not search in ", procFile).Base(err)
	}
	if inode == "" {
		return 0, "", "", errors.New("connection for ", dest.Address, ":", dest.Port, " not found in ", procFile)
	}

	pidStr, err := findPidByInode(inode)
	if err != nil {
		return 0, "", "", errors.New("could not find PID for inode ", inode, ": ", err)
	}
	if pidStr == "" {
		return 0, "", "", errors.New("no process found for inode ", inode)
	}

	absPath, err := getAbsPath(pidStr)
	if err != nil {
		return 0, "", "", errors.New("could not get process name for PID ", pidStr, ":", err)
	}

	nameSplit := strings.Split(absPath, "/")
	procName := nameSplit[len(nameSplit)-1]

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, "", "", errors.New("failed to parse PID: ", err)
	}

	return pid, procName, absPath, nil
}

func formatLittleEndianString(addr Address, port Port) (string, error) {
	ip := addr.IP()
	var ipBytes []byte
	if addr.Family() == AddressFamilyIPv4 {
		ipBytes = ip.To4()
	} else {
		ipBytes = ip.To16()
	}
	if ipBytes == nil {
		return "", errors.New("invalid IP format for ", addr.Family(), ": ", ip)
	}

	for i, j := 0, len(ipBytes)-1; i < j; i, j = i+1, j-1 {
		ipBytes[i], ipBytes[j] = ipBytes[j], ipBytes[i]
	}
	portHex := fmt.Sprintf("%04X", uint16(port))
	ipHex := strings.ToUpper(hex.EncodeToString(ipBytes))
	return fmt.Sprintf("%s:%s", ipHex, portHex), nil
}

func findInodeInFile(filePath, targetHexAddr string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 10 {
			continue
		}

		localAddress := fields[1]
		if localAddress == targetHexAddr {
			inode := fields[9]
			return inode, nil
		}
	}

	return "", scanner.Err()
}

func findPidByInode(inode string) (string, error) {
	procDir, err := os.ReadDir("/proc")
	if err != nil {
		return "", err
	}

	targetLink := "socket:[" + inode + "]"

	for _, entry := range procDir {
		if !entry.IsDir() {
			continue
		}
		pid := entry.Name()
		if _, err := strconv.Atoi(pid); err != nil {
			continue
		}

		fdPath := fmt.Sprintf("/proc/%s/fd", pid)
		fdDir, err := os.ReadDir(fdPath)
		if err != nil {
			continue
		}

		for _, fdEntry := range fdDir {
			linkPath := fmt.Sprintf("%s/%s", fdPath, fdEntry.Name())
			linkTarget, err := os.Readlink(linkPath)
			if err != nil {
				continue
			}
			if linkTarget == targetLink {
				return pid, nil
			}
		}
	}
	return "", nil
}

func getAbsPath(pid string) (string, error) {
	path := fmt.Sprintf("/proc/%s/exe", pid)
	return os.Readlink(path)
}
