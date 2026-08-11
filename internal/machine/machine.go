// Package machine 采集本机硬件信息生成机器码。
//
// Linux 下直接读取 sysfs（零依赖）：
//   - 主板序列号: /sys/class/dmi/id/board_serial
//   - CPU 序列号: /proc/cpuinfo 的 Serial 字段（x86 通常为空，退化为型号+核心数）
//   - 主 MAC:     /sys/class/net/*/address（过滤虚拟网卡）
//   - 磁盘序列号: /sys/block/*/device/serial（取第一块非虚拟磁盘）
//   - 系统 UUID:  /sys/class/dmi/id/product_uuid（虚拟机场景唯一标识，克隆 VM 也唯一）
//
// 机器码 = SHA-256(主板|CPU|MAC|磁盘|UUID[|盐])。UUID 是 VM 场景的区分度关键：
// KVM/VMware/OpenStack 均注入唯一 UUID，弥补 VM 下主板/磁盘序列号常为空、CPU 退化的短板。
package machine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

// HardwareInfo 采集到的原始硬件信息（供展示/排查）
type HardwareInfo struct {
	MotherboardSerial string
	CPUSerial         string
	MacAddress        string
	DiskSerial        string
	SystemUUID        string
	MachineCode       string
}

func (h HardwareInfo) String() string {
	return fmt.Sprintf("主板序列号=%s, CPU序列号=%s, 主MAC=%s, 磁盘序列号=%s, 系统UUID=%s, 机器码=%s",
		h.MotherboardSerial, h.CPUSerial, h.MacAddress, h.DiskSerial, h.SystemUUID, h.MachineCode)
}

// Collect 采集本机硬件信息并计算机器码
func Collect(salt string) HardwareInfo {
	mb := readTrim("/sys/class/dmi/id/board_serial")
	cpu := cpuSerial()
	mac := primaryMac()
	disk := primaryDiskSerial()
	uuid := readTrim("/sys/class/dmi/id/product_uuid")

	machineCode := compute(mb, cpu, mac, disk, uuid, salt)
	return HardwareInfo{
		MotherboardSerial: mb,
		CPUSerial:         cpu,
		MacAddress:        mac,
		DiskSerial:        disk,
		SystemUUID:        uuid,
		MachineCode:       machineCode,
	}
}

// Generate 仅返回机器码
func Generate(salt string) string {
	return Collect(salt).MachineCode
}

func compute(mb, cpu, mac, disk, uuid, salt string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s", mb, cpu, mac, disk, uuid)
	if salt != "" {
		fmt.Fprintf(h, "|%s", salt)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func readTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// cpuSerial 读取 CPU 序列号；x86 无 Serial 字段时返回型号+核心数（保证不同机器差异）
func cpuSerial() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	serial := ""
	model := ""
	cores := 0
	for _, line := range strings.Split(string(data), "\n") {
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "Serial":
			if serial == "" {
				serial = val
			}
		case "model name":
			if model == "" {
				model = val
			}
		case "processor":
			cores++
		}
	}
	if serial != "" {
		return serial
	}
	return fmt.Sprintf("%s-%dcores", model, cores)
}

// primaryMac 选取主 MAC：排除虚拟网卡（lo/veth/docker/virbr/br-/tun/tap/vnet/wg），按速率排序取最大
func primaryMac() string {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return ""
	}
	type iface struct {
		name  string
		speed string
	}
	var candidates []iface
	for _, e := range entries {
		name := e.Name()
		if isVirtualIf(name) {
			continue
		}
		addr := readTrim("/sys/class/net/" + name + "/address")
		if addr == "" || addr == "00:00:00:00:00:00" {
			continue
		}
		speed := readTrim("/sys/class/net/" + name + "/speed")
		candidates = append(candidates, iface{name: name, speed: speed})
	}
	if len(candidates) == 0 {
		return ""
	}
	// 按速率降序，速率相同按名称排序保证确定性
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].speed != candidates[j].speed {
			return candidates[i].speed > candidates[j].speed
		}
		return candidates[i].name < candidates[j].name
	})
	return readTrim("/sys/class/net/" + candidates[0].name + "/address")
}

func isVirtualIf(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range []string{"lo", "veth", "docker", "virbr", "br-", "tun", "tap", "vnet", "wg"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// primaryDiskSerial 取第一块物理磁盘序列号（排除 loop/ram/zram 虚拟设备）
func primaryDiskSerial() string {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return ""
	}
	var serials []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "zram") || strings.HasPrefix(name, "dm-") {
			continue
		}
		// 虚拟设备（nvme 子设备等）serial 为空则跳过
		s := readTrim("/sys/block/" + name + "/device/serial")
		if s != "" {
			serials = append(serials, s)
		}
	}
	sort.Strings(serials)
	if len(serials) == 0 {
		return ""
	}
	return serials[0]
}
